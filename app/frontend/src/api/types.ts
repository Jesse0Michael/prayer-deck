import type { components } from "./schema";

type Schemas = components["schemas"];

export type User = Schemas["User"];
export type Deck = Schemas["Deck"];
export type Card = Schemas["Card"];
export type CardKind = Schemas["CardKind"];
export type CardInput = Schemas["CardInput"];
export type Artwork = Schemas["Artwork"];
export type Draw = Schemas["Draw"];
export type Tokens = Schemas["Tokens"];
export type LoginProvider = Schemas["LoginProviders"]["providers"][number];
