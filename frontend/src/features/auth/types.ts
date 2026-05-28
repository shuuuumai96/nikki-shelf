export type AuthUser = {
  id: number;
  username: string;
  role: "owner" | "user";
  csrfToken?: string;
};

export type SignupMode = "setup" | "open" | "closed";

export type AuthConfig = {
  signupMode: SignupMode;
  signupAvailable: boolean;
};

export type AuthCredentials = {
  username: string;
  password: string;
};
