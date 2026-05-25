"""Format or check Nikki frontend and backend source files."""

from __future__ import annotations

import argparse
import os
import shlex
import shutil
import subprocess
import sys
import time
from pathlib import Path


MODULE_PATH = "github.com/shuuuumai96/nikki-shelf/backend"
GOIMPORTS = "golang.org/x/tools/cmd/goimports@v0.45.0"


class Console:
    """Small ANSI-aware terminal output helper."""

    def __init__(self, color: bool) -> None:
        self.color = color
        self.unicode = stream_can_encode(sys.stdout, "✓✗─")
        self.rule_char = "─" if self.unicode else "-"
        self.ok_symbol = "✓" if self.unicode else "OK"
        self.fail_symbol = "✗" if self.unicode else "x"

    def paint(self, text: str, code: str) -> str:
        if not self.color:
            return text
        return f"\x1b[{code}m{text}\x1b[0m"

    def dim(self, text: str) -> str:
        return self.paint(text, "2")

    def green(self, text: str) -> str:
        return self.paint(text, "32")

    def red(self, text: str) -> str:
        return self.paint(text, "31")

    def yellow(self, text: str) -> str:
        return self.paint(text, "33")

    def cyan(self, text: str) -> str:
        return self.paint(text, "36")

    def rule(self, title: str) -> None:
        width = shutil.get_terminal_size((88, 20)).columns
        label = f" {title} "
        print()
        print(self.cyan(label + self.rule_char * max(8, width - len(label))))

    def info(self, label: str, value: str) -> None:
        print(f"{self.dim(label + ':'):>14} {value}")

    def ok(self, message: str) -> None:
        print(f"{self.green(self.ok_symbol)} {message}")

    def warn(self, message: str) -> None:
        print(f"{self.yellow('!')} {message}")

    def fail(self, message: str) -> None:
        print(f"{self.red(self.fail_symbol)} {message}")

    def command(self, command: list[str]) -> None:
        rendered = " ".join(shlex.quote(part) for part in command)
        print(f"{self.dim('$')} {self.cyan(rendered)}")


def should_use_color(mode: str) -> bool:
    if mode == "always":
        return True
    if mode == "never":
        return False
    if os.environ.get("NO_COLOR"):
        return False
    if os.environ.get("FORCE_COLOR"):
        return True
    if os.environ.get("TERM") == "dumb":
        return False
    return sys.stdout.isatty()


def stream_can_encode(stream: object, text: str) -> bool:
    encoding = getattr(stream, "encoding", None) or "utf-8"

    try:
        text.encode(encoding)
    except UnicodeEncodeError:
        return False

    return True


def run(command: list[str], cwd: Path, console: Console) -> None:
    """Run a command and print its working directory, command line, and duration."""
    console.info("cwd", str(cwd))
    console.command(command)

    started = time.perf_counter()
    resolved = shutil.which(command[0])
    command_to_run = [resolved or command[0], *command[1:]]

    try:
        subprocess.run(command_to_run, cwd=cwd, check=True)
    except subprocess.CalledProcessError as exc:
        elapsed = time.perf_counter() - started
        console.fail(f"failed after {elapsed:.2f}s with exit code {exc.returncode}")
        raise

    elapsed = time.perf_counter() - started
    console.ok(f"completed in {elapsed:.2f}s")


def collect_go_files(backend_dir: Path) -> list[str]:
    """Return Go files that should be checked by goimports."""
    files: list[str] = []

    for path in backend_dir.rglob("*.go"):
        if not path.is_file():
            continue

        parts = set(path.parts)

        if "vendor" in parts:
            continue
        if "tmp" in parts:
            continue
        if path.name.endswith(".pb.go"):
            continue

        files.append(str(path.resolve()))

    return files


def format_frontend(frontend_dir: Path, check: bool, console: Console) -> None:
    """Run the frontend formatter or formatter check."""
    console.rule("Frontend format check" if check else "Frontend format")

    command = ["corepack", "pnpm", "format:check" if check else "format"]
    run(command, cwd=frontend_dir, console=console)


def format_backend(backend_dir: Path, check: bool, console: Console) -> None:
    """Run goimports or report files that need goimports."""
    console.rule("Backend goimports check" if check else "Backend goimports")

    go_files = collect_go_files(backend_dir)
    console.info("go files", str(len(go_files)))

    if not go_files:
        console.warn("no Go files found")
        return

    command = ["go", "run", GOIMPORTS, "-local", MODULE_PATH]

    if check:
        check_command = [*command, "-l", *go_files]
        console.info("cwd", str(backend_dir))
        console.command(check_command)

        started = time.perf_counter()

        try:
            result = subprocess.run(
                check_command,
                cwd=backend_dir,
                text=True,
                capture_output=True,
                check=True,
            )
        except subprocess.CalledProcessError as exc:
            elapsed = time.perf_counter() - started
            console.fail(f"goimports check command failed after {elapsed:.2f}s")

            if exc.stdout:
                print(exc.stdout, end="")
            if exc.stderr:
                print(exc.stderr, end="", file=sys.stderr)

            raise

        unformatted = [
            line.strip() for line in result.stdout.splitlines() if line.strip()
        ]

        elapsed = time.perf_counter() - started

        if unformatted:
            console.fail(
                f"{len(unformatted)} backend Go file(s) need goimports "
                f"after {elapsed:.2f}s"
            )
            for path in unformatted:
                print(f"  {console.red(path)}")
            sys.exit(1)

        console.ok(f"goimports check passed in {elapsed:.2f}s")
        return

    run([*command, "-w", *go_files], cwd=backend_dir, console=console)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Format or check Nikki frontend/backend code."
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Check formatting without modifying files.",
    )
    parser.add_argument(
        "--color",
        choices=["auto", "always", "never"],
        default="auto",
        help="Control ANSI color output.",
    )

    args = parser.parse_args()
    console = Console(color=should_use_color(args.color))

    script_dir = Path(__file__).resolve().parent
    repo_root = script_dir.parent
    frontend_dir = repo_root / "frontend"
    backend_dir = repo_root / "backend"

    started = time.perf_counter()

    console.rule("Nikki format")
    console.info("mode", "check" if args.check else "write")
    console.info("repo", str(repo_root))

    try:
        format_frontend(frontend_dir, args.check, console)
        format_backend(backend_dir, args.check, console)
    except subprocess.CalledProcessError as exc:
        elapsed = time.perf_counter() - started
        console.rule("Summary")
        console.fail(f"format task failed in {elapsed:.2f}s")
        return exc.returncode or 1

    elapsed = time.perf_counter() - started

    console.rule("Summary")
    console.ok(f"all format tasks completed in {elapsed:.2f}s")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
