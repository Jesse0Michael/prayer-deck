import { motion } from "motion/react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { ApiClient } from "../api/client";
import type { Card, CardKind } from "../api/types";
import { CardFace } from "../components/CardFace";

interface CardEditorProps {
  api: ApiClient;
  deckID: string;
  /** The card to edit; omitted when adding a new card. */
  card?: Card;
  onSaved: (card: Card) => void;
  onDeleted: (cardID: string) => void;
  onClose: () => void;
}

// Inline because the CSS minifier keeps only the -webkit- form of backdrop-filter.
const blur = { backdropFilter: "blur(4px)", WebkitBackdropFilter: "blur(4px)" };

const kinds: { value: CardKind; label: string }[] = [
  { value: "person", label: "Person" },
  { value: "couple", label: "Couple" },
  { value: "family", label: "Family" },
];

export function CardEditor({ api, deckID, card, onSaved, onDeleted, onClose }: CardEditorProps) {
  const [name, setName] = useState(card?.name ?? "");
  const [kind, setKind] = useState<CardKind>(card?.kind ?? "person");
  const [details, setDetails] = useState(card?.details ?? "");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!file) return;
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && !saving && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, saving]);

  const chooseFile = (f: File | undefined) => {
    if (!f) return;
    if (!f.type.startsWith("image/")) {
      setError("Choose an image file (PNG, JPEG, WebP, or GIF).");
      return;
    }
    setError(null);
    setFile(f);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError("Give this card a name.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const input = { name: name.trim(), kind, details: details.trim() };
      let saved = card ? await api.updateCard(card.id, input) : await api.createCard(deckID, input);
      if (file) saved = await api.uploadArtwork(saved.id, file);
      onSaved(saved);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save the card.");
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!card) return;
    setSaving(true);
    try {
      await api.deleteCard(card.id);
      onDeleted(card.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not delete the card.");
      setSaving(false);
    }
  };

  const previewCard: Card | null = preview
    ? {
        id: "preview",
        name: name || "New card",
        kind,
        details,
        artwork: { display_url: preview, thumbnail_url: preview, original_url: preview, width: 0, height: 0 },
        created_at: "",
        updated_at: "",
      }
    : card
      ? { ...card, name: name || card.name, kind }
      : { id: "new", name: name || "New card", kind, details, artwork: null, created_at: "", updated_at: "" };

  return (
    <motion.div className="sheet-backdrop" style={blur} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={() => !saving && onClose()}>
      <motion.form
        className="sheet"
        onSubmit={submit}
        onClick={(e) => e.stopPropagation()}
        initial={{ y: 40, opacity: 0, scale: 0.98 }}
        animate={{ y: 0, opacity: 1, scale: 1 }}
        exit={{ y: 30, opacity: 0, scale: 0.98 }}
        transition={{ type: "spring", stiffness: 220, damping: 26 }}
      >
        <h2>{card ? "Edit card" : "New card"}</h2>
        <div className="editor-grid">
          <div
            className={`artwork-drop${dragging ? " dragging" : ""}`}
            onClick={() => fileInput.current?.click()}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              chooseFile(e.dataTransfer.files[0]);
            }}
            role="button"
            tabIndex={0}
            aria-label="Choose artwork"
          >
            {previewCard && (
              <div className="artwork-preview">
                <CardFace card={previewCard} />
              </div>
            )}
            <span className="artwork-hint">{previewCard?.artwork ? "Replace artwork" : "Add artwork"}</span>
            <input ref={fileInput} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onChange={(e) => chooseFile(e.target.files?.[0])} />
          </div>

          <div className="editor-fields">
            <label className="field">
              <span>Name</span>
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Rhett, or The Garcias" autoFocus={!card} maxLength={80} />
            </label>

            <fieldset className="field">
              <legend>This card is for</legend>
              <div className="segmented">
                {kinds.map((k) => (
                  <label key={k.value} className={kind === k.value ? "selected" : undefined}>
                    <input type="radio" name="kind" value={k.value} checked={kind === k.value} onChange={() => setKind(k.value)} />
                    {k.label}
                  </label>
                ))}
              </div>
            </fieldset>

            <label className="field">
              <span>Little details</span>
              <textarea
                value={details}
                onChange={(e) => setDetails(e.target.value)}
                rows={4}
                placeholder="A hobby, a pet, a favorite place — whatever brings them to mind."
                maxLength={1000}
              />
            </label>
          </div>
        </div>

        {error && <p className="error-text">{error}</p>}

        <div className="sheet-actions">
          {card &&
            (confirmDelete ? (
              <span className="confirm-delete">
                Remove {card.name}?
                <button type="button" className="btn btn-danger" onClick={remove} disabled={saving}>
                  Remove
                </button>
                <button type="button" className="btn btn-quiet" onClick={() => setConfirmDelete(false)} disabled={saving}>
                  Keep
                </button>
              </span>
            ) : (
              <button type="button" className="btn btn-quiet btn-danger-text" onClick={() => setConfirmDelete(true)} disabled={saving}>
                Remove card
              </button>
            ))}
          <span className="spacer" />
          <button type="button" className="btn btn-quiet" onClick={onClose} disabled={saving}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </motion.form>
    </motion.div>
  );
}
