import type { components } from "./schema";

type Schemas = components["schemas"];

export type User = Schemas["User"];
export type Deck = Schemas["Deck"];
export type Card = Schemas["Card"];
export type CardInput = Schemas["CardInput"];
export type Note = Schemas["Note"];
export type Media = Schemas["Media"];
export type DrawnCard = Schemas["DrawnCard"];
export type Draw = Schemas["Draw"];
/** A card that may carry the face it was dealt with; cards outside a draw show their primary media. */
export type DealtCard = Card & { face?: Media | null };
export type Tokens = Schemas["Tokens"];
export type LoginProvider = Schemas["LoginProviders"]["providers"][number];
