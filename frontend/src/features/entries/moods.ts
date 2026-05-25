import { Cloud, Leaf, Moon, Sparkles, Sun } from "lucide-vue-next";
import type { Component } from "vue";
import type { MoodKey } from "./types";

export type MoodSpec = {
  key: MoodKey;
  label: string;
  labelKey: string;
  color: string;
  icon: Component;
};

export const moodSpecs: Record<MoodKey, MoodSpec> = {
  happy: {
    key: "happy",
    label: "うれしい",
    labelKey: "entries.moodHappy",
    color: "#F7E7A6",
    icon: Sun,
  },
  calm: {
    key: "calm",
    label: "おだやか",
    labelKey: "entries.moodCalm",
    color: "#BFE8D4",
    icon: Leaf,
  },
  tired: {
    key: "tired",
    label: "つかれた",
    labelKey: "entries.moodTired",
    color: "#D8CEF6",
    icon: Moon,
  },
  sad: {
    key: "sad",
    label: "しんみり",
    labelKey: "entries.moodSad",
    color: "#BFDDF3",
    icon: Cloud,
  },
  excited: {
    key: "excited",
    label: "わくわく",
    labelKey: "entries.moodExcited",
    color: "#CFE5F7",
    icon: Sparkles,
  },
};

export const moodOrder: MoodKey[] = [
  "happy",
  "calm",
  "tired",
  "sad",
  "excited",
];
