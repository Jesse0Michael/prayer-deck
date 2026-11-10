import { AnimatePresence, motion } from "motion/react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { ApiClient } from "../api/client";
import type { Card, Media } from "../api/types";
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

/** A picture in the editor. A picked file carries a local preview until Save uploads it. */
interface EditorImage {
  media: Media;
  file?: File;
}

// Two rows of three, so a card with hundreds of pictures stays a tidy grid.
const GALLERY_PAGE_SIZE = 6;

// Inline because the CSS minifier keeps only the -webkit- form of backdrop-filter.
const blur = { backdropFilter: "blur(4px)", WebkitBackdropFilter: "blur(4px)" };

function pendingImage(file: File): EditorImage {
  const url = URL.createObjectURL(file);
  return {
    file,
    media: {
      id: `pending-${crypto.randomUUID()}`,
      kind: "image",
      display_url: url,
      thumbnail_url: url,
      original_url: url,
      width: 0,
      height: 0,
      created_at: "",
    },
  };
}

export function CardEditor({ api, deckID, card, onSaved, onDeleted, onClose }: CardEditorProps) {
  const [name, setName] = useState(card?.name ?? "");
  const [notes, setNotes] = useState(card?.notes ?? "");
  // Newest first: the first image is the card's primary face, the rest fill the gallery.
  const [images, setImages] = useState<EditorImage[]>(() => (card?.media ?? []).map((media) => ({ media })));
  const [removed, setRemoved] = useState<string[]>([]);
  const [page, setPage] = useState(0);
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const previewURLs = useRef(new Set<string>());
  // The server's latest copy of the card after a partly finished save, so closing still shows what was stored.
  const latest = useRef<Card | null>(null);

  useEffect(() => {
    const urls = previewURLs.current;
    return () => urls.forEach((url) => URL.revokeObjectURL(url));
  }, []);

  const close = () => {
    if (latest.current) onSaved(latest.current);
    else onClose();
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && !saving && close();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const addFiles = (files: FileList | null | undefined) => {
    const picked = Array.from(files ?? []);
    if (picked.length === 0) return;
    const usable = picked.filter((f) => f.type.startsWith("image/"));
    setError(usable.length < picked.length ? "Choose image files (PNG, JPEG, WebP, or GIF)." : null);
    if (usable.length === 0) return;
    const added = usable.map(pendingImage);
    added.forEach((img) => previewURLs.current.add(img.media.display_url));
    setImages((prev) => [...added, ...prev]);
    setPage(0);
  };

  const removeImage = (img: EditorImage) => {
    if (img.file) {
      URL.revokeObjectURL(img.media.display_url);
      previewURLs.current.delete(img.media.display_url);
    } else {
      setRemoved((prev) => [...prev, img.media.id]);
    }
    setImages((prev) => prev.filter((i) => i !== img));
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError("Give this card a name.");
      return;
    }
    setSaving(true);
    setError(null);
    const input = { name: name.trim(), notes: notes.trim() };
    try {
      let saved = latest.current ?? card ?? (await api.createCard(deckID, input));
      latest.current = saved;
      for (const mediaID of removed) {
        await api.deleteMedia(saved.id, mediaID);
        setRemoved((prev) => prev.filter((id) => id !== mediaID));
      }
      // Oldest first, so the image shown as primary is the last uploaded and lands first.
      for (const img of [...images].reverse()) {
        if (!img.file) continue;
        saved = await api.addMedia(saved.id, img.file);
        latest.current = saved;
        const stored = saved.media[0];
        setImages((prev) => prev.map((i) => (i === img ? { media: stored } : i)));
      }
      onSaved(await api.updateCard(saved.id, input));
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

  const primary = images[0]?.media ?? null;
  const previewCard: Card = { id: card?.id ?? "new", name: name || card?.name || "New card", notes, media: [], created_at: "", updated_at: "" };
  const gallery = images.slice(1);
  const pageCount = Math.ceil(gallery.length / GALLERY_PAGE_SIZE);
  const currentPage = Math.min(page, Math.max(pageCount - 1, 0));
  const pageImages = gallery.slice(currentPage * GALLERY_PAGE_SIZE, (currentPage + 1) * GALLERY_PAGE_SIZE);

  return (
    <motion.div className="sheet-backdrop" style={blur} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={() => !saving && close()}>
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
          <div className="artwork-column">
            <div
              className={`artwork-drop${dragging ? " dragging" : ""}`}
              onClick={() => fileInput.current?.click()}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  fileInput.current?.click();
                }
              }}
              onDragOver={(e) => {
                e.preventDefault();
                setDragging(true);
              }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => {
                e.preventDefault();
                setDragging(false);
                addFiles(e.dataTransfer.files);
              }}
              role="button"
              tabIndex={0}
              aria-label="Add image"
            >
              <div className="artwork-preview">
                <CardFace card={previewCard} media={primary} />
              </div>
              <span className="artwork-hint">Add image</span>
              <input
                ref={fileInput}
                type="file"
                accept="image/png,image/jpeg,image/webp,image/gif"
                multiple
                hidden
                onChange={(e) => {
                  addFiles(e.target.files);
                  e.target.value = "";
                }}
              />
            </div>

            {gallery.length > 0 && (
              <div className="gallery">
                <ul className="gallery-grid">
                  <AnimatePresence initial={false} mode="popLayout">
                    {pageImages.map((img) => (
                      <motion.li
                        key={img.media.id}
                        className="gallery-item"
                        layout
                        initial={{ opacity: 0, scale: 0.85 }}
                        animate={{ opacity: 1, scale: 1 }}
                        exit={{ opacity: 0, scale: 0.85 }}
                        transition={{ type: "spring", stiffness: 300, damping: 26 }}
                      >
                        <img src={img.media.thumbnail_url} alt="" draggable={false} decoding="async" />
                        <button
                          type="button"
                          className="gallery-remove"
                          onClick={() => removeImage(img)}
                          disabled={saving}
                          aria-label="Remove image"
                        >
                          ×
                        </button>
                      </motion.li>
                    ))}
                  </AnimatePresence>
                </ul>
                {pageCount > 1 && (
                  <div className="gallery-pages">
                    <button
                      type="button"
                      className="gallery-page-btn"
                      onClick={() => setPage(currentPage - 1)}
                      disabled={currentPage === 0}
                      aria-label="Previous images"
                    >
                      ‹
                    </button>
                    <span>
                      {currentPage + 1} / {pageCount}
                    </span>
                    <button
                      type="button"
                      className="gallery-page-btn"
                      onClick={() => setPage(currentPage + 1)}
                      disabled={currentPage === pageCount - 1}
                      aria-label="Next images"
                    >
                      ›
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="editor-fields">
            <label className="field">
              <span>Name</span>
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Rhett, or The Seins" autoFocus={!card} maxLength={80} />
            </label>

            <label className="field">
              <span>Notes</span>
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={4}
                placeholder="How can you pray for them? What can you thank God for in their lives or the work they do?"
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
          <button type="button" className="btn btn-quiet" onClick={close} disabled={saving}>
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
