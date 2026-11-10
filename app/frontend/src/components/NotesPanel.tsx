import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import type { DeckApi } from "../api/deckApi";
import type { Note } from "../api/types";

interface NotesPanelProps {
  api: DeckApi;
  /** Null until the card exists; the first note then calls createCard. */
  cardID: string | null;
  createCard?: () => Promise<string>;
  /** Notes shown per page. */
  pageSize: number;
}

const NOTE_MAX_LENGTH = 2000;

const dayFormat = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
});
const dayYearFormat = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  year: "numeric",
});
const fullFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
});

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

/** Today and yesterday read as words; older dates drop the year when it is the current one. */
function friendlyDate(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const days = Math.round((startOfDay(now) - startOfDay(date)) / 86_400_000);
  if (days === 0) return "Today";
  if (days === 1) return "Yesterday";
  return (
    date.getFullYear() === now.getFullYear() ? dayFormat : dayYearFormat
  ).format(date);
}

/** Saves on Cmd/Ctrl+Enter so a long note never needs the pointer. */
function submitOnModEnter(
  e: KeyboardEvent<HTMLTextAreaElement>,
  submit: () => void,
) {
  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    submit();
  }
}

/** Lets a parent form save a note still being typed. */
export interface NotesPanelHandle {
  /** Saves the unsaved new note, if any. Resolves false when saving failed. */
  saveDraft: () => Promise<boolean>;
}

/** A card's notes: a form to add one, then the notes newest first, a page at a time. */
export const NotesPanel = forwardRef<NotesPanelHandle, NotesPanelProps>(
  function NotesPanel({ api, cardID, createCard, pageSize }, ref) {
    const [notes, setNotes] = useState<Note[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(0);
    const [draft, setDraft] = useState("");
    const [editingID, setEditingID] = useState<string | null>(null);
    const [editText, setEditText] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);
    // Only the latest load may update the list, so quickly paging back and forth never shows a stale page.
    const loadSeq = useRef(0);

    const load = useCallback(
      async (target: number) => {
        if (!cardID) return;
        const seq = ++loadSeq.current;
        try {
          const result = await api.notes(cardID, pageSize, target * pageSize);
          if (seq !== loadSeq.current) return;
          const lastPage = Math.max(Math.ceil(result.total / pageSize) - 1, 0);
          // Deleting a page's last note leaves it empty, so step back to the new last page.
          if (target > lastPage) return load(lastPage);
          setNotes(result.notes);
          setTotal(result.total);
          setPage(target);
          setError(null);
        } catch (err) {
          if (seq === loadSeq.current)
            setError(
              err instanceof Error ? err.message : "Could not load notes.",
            );
        }
      },
      [api, cardID, pageSize],
    );

    useEffect(() => {
      setNotes([]);
      setTotal(0);
      setEditingID(null);
      setDraft("");
      void load(0);
    }, [load]);

    const run = async (action: () => Promise<void>, failure: string) => {
      setBusy(true);
      try {
        await action();
      } catch (err) {
        setError(err instanceof Error ? err.message : failure);
      } finally {
        setBusy(false);
      }
    };

    const saveDraft = async (): Promise<boolean> => {
      const text = draft.trim();
      if (!text) return true;
      setBusy(true);
      try {
        const id = cardID ?? (await createCard?.());
        if (!id) return false;
        await api.createNote(id, text);
        setDraft("");
        setEditingID(null);
        // A card created by this note reloads on its own once cardID changes.
        if (id === cardID) await load(0);
        return true;
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Could not save the note.",
        );
        return false;
      } finally {
        setBusy(false);
      }
    };

    useImperativeHandle(ref, () => ({ saveDraft }));

    const add = () => void saveDraft();

    const saveEdit = (note: Note) => {
      const text = editText.trim();
      if (!text) return;
      void run(async () => {
        const saved = await api.updateNote(note.id, text);
        setNotes((prev) => prev.map((n) => (n.id === saved.id ? saved : n)));
        setEditingID(null);
      }, "Could not save the note.");
    };

    const remove = (note: Note) => {
      void run(async () => {
        await api.deleteNote(note.id);
        if (editingID === note.id) setEditingID(null);
        await load(page);
      }, "Could not delete the note.");
    };

    const pageCount = Math.ceil(total / pageSize);

    return (
      <section className="notes-panel" aria-label="Notes">
        <div className="note-form">
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => submitOnModEnter(e, add)}
            rows={3}
            placeholder="How can you pray for them? What can you thank God for in their lives or the work they do?"
            maxLength={NOTE_MAX_LENGTH}
            aria-label="New note"
          />
          <button
            type="button"
            className="note-btn note-btn-save"
            onClick={add}
            disabled={busy || !draft.trim()}
            aria-label="Save note"
          >
            ✓
          </button>
        </div>

        {error && <p className="error-text">{error}</p>}

        {notes.length > 0 && (
          <ul className="note-list">
            {notes.map((note) => (
              <li key={note.id} className="note-item">
                {editingID === note.id ? (
                  <>
                    <textarea
                      className="note-edit"
                      value={editText}
                      onChange={(e) => setEditText(e.target.value)}
                      onKeyDown={(e) =>
                        submitOnModEnter(e, () => saveEdit(note))
                      }
                      rows={3}
                      maxLength={NOTE_MAX_LENGTH}
                      aria-label="Edit note"
                      autoFocus
                    />
                    <div className="note-actions">
                      <button
                        type="button"
                        className="note-btn note-btn-save"
                        onClick={() => saveEdit(note)}
                        disabled={busy || !editText.trim()}
                        aria-label="Save changes"
                      >
                        ✓
                      </button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="note-body">
                      <time
                        className="note-date"
                        dateTime={note.created_at}
                        title={fullFormat.format(new Date(note.created_at))}
                      >
                        {friendlyDate(note.created_at)}
                      </time>
                      <p className="note-text">{note.text}</p>
                    </div>
                    <div className="note-actions">
                      <button
                        type="button"
                        className="note-btn"
                        onClick={() => {
                          setEditingID(note.id);
                          setEditText(note.text);
                        }}
                        disabled={busy}
                        aria-label="Edit note"
                      >
                        ✎
                      </button>
                      <button
                        type="button"
                        className="note-btn note-btn-delete"
                        onClick={() => remove(note)}
                        disabled={busy}
                        aria-label="Delete note"
                      >
                        ×
                      </button>
                    </div>
                  </>
                )}
              </li>
            ))}
          </ul>
        )}

        {pageCount > 1 && (
          <div className="gallery-pages">
            <button
              type="button"
              className="gallery-page-btn"
              onClick={() => void load(page - 1)}
              disabled={page === 0}
              aria-label="Newer notes"
            >
              ‹
            </button>
            <span>
              {page + 1} / {pageCount}
            </span>
            <button
              type="button"
              className="gallery-page-btn"
              onClick={() => void load(page + 1)}
              disabled={page === pageCount - 1}
              aria-label="Older notes"
            >
              ›
            </button>
          </div>
        )}
      </section>
    );
  },
);
