package labs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
)

// Virtual files are the lab editor's manifests (simulator-compatibility.md §5). They live
// with the lab, are read by kubectl -f, and start from the lesson's initialFiles.

// File limits per lab.
const (
	MaxFiles     = 20
	MaxFileBytes = 64 << 10
)

// FilePath is the shape of a virtual path (lab_files.path CHECK, OpenAPI VirtualPath).
var FilePath = regexp.MustCompile(`^manifests/[a-z0-9][a-z0-9._-]{0,62}\.ya?ml$`)

// Errors from file operations.
var (
	// ErrInvalidFile: bad path, too large, or not UTF-8 text.
	ErrInvalidFile = errors.New("invalid file")
	// ErrFileLimit: the lab already has MaxFiles files.
	ErrFileLimit = errors.New("lab file limit reached")
	// ErrStaleFile: the file changed after the client loaded it (baseUpdatedAt mismatch).
	ErrStaleFile = errors.New("file changed since it was loaded")
	// ErrFileNotFound: the lab has no file at that path.
	ErrFileNotFound = errors.New("file not found")
)

// LabFile mirrors the OpenAPI LabFile.
type LabFile struct {
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	SizeBytes int       `json:"sizeBytes"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func fileFromRow(r dbgen.LabFile) LabFile {
	return LabFile{Path: r.Path, Content: r.Content, SizeBytes: len(r.Content), UpdatedAt: r.UpdatedAt}
}

// validFile reports whether a file can be stored.
func validFile(path, content string) bool {
	if !FilePath.MatchString(path) || len(content) > MaxFileBytes || !utf8.ValidString(content) {
		return false
	}
	for i := 0; i < len(content); i++ {
		if content[i] == 0 {
			return false
		}
	}
	return true
}

// Files returns the lab's files in path order.
func (sv *Service) Files(ctx context.Context, userID, labID uuid.UUID) ([]LabFile, error) {
	q := dbgen.New(sv.pool)
	if _, err := q.GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := q.ListLabFiles(ctx, labID)
	if err != nil {
		return nil, err
	}
	out := make([]LabFile, 0, len(rows))
	for _, r := range rows {
		out = append(out, fileFromRow(r))
	}
	return out, nil
}

// SaveFile creates or replaces a file in an active lab. If base is set, the save fails with
// ErrStaleFile unless the stored file was last updated at base (another tab saved since).
// Files aren't cluster state, so saving doesn't change the lab version.
func (sv *Service) SaveFile(ctx context.Context, userID, labID uuid.UUID, path, content string, base *time.Time) (LabFile, error) {
	if !validFile(path, content) {
		return LabFile{}, ErrInvalidFile
	}
	var saved LabFile
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		existing, err := q.GetLabFile(ctx, dbgen.GetLabFileParams{SessionID: s.ID, Path: path})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if base != nil {
				return Change{}, ErrStaleFile
			}
			n, err := q.CountLabFiles(ctx, s.ID)
			if err != nil {
				return Change{}, err
			}
			if n >= MaxFiles {
				return Change{}, ErrFileLimit
			}
		case err != nil:
			return Change{}, err
		case base != nil && !existing.UpdatedAt.Equal(*base):
			return Change{}, ErrStaleFile
		}
		row, err := q.UpsertLabFile(ctx, dbgen.UpsertLabFileParams{SessionID: s.ID, Path: path, Content: content})
		if err != nil {
			return Change{}, fmt.Errorf("save file: %w", err)
		}
		saved = fileFromRow(row)
		return Change{}, nil
	})
	return saved, err
}

// DeleteFile removes a file from an active lab.
func (sv *Service) DeleteFile(ctx context.Context, userID, labID uuid.UUID, path string) error {
	if !FilePath.MatchString(path) {
		return ErrInvalidFile
	}
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		n, err := q.DeleteLabFile(ctx, dbgen.DeleteLabFileParams{SessionID: s.ID, Path: path})
		if err != nil {
			return Change{}, err
		}
		if n == 0 {
			return Change{}, ErrFileNotFound
		}
		return Change{}, nil
	})
	return err
}

// seedFiles replaces the lab's files with the lesson's initial files (on create and reset).
func seedFiles(ctx context.Context, q *dbgen.Queries, labID uuid.UUID, files []VirtualFile) error {
	if err := q.DeleteAllLabFiles(ctx, labID); err != nil {
		return err
	}
	for _, f := range files {
		if !validFile(f.Path, f.Content) {
			// Content is validated at import; failing here is a content bug.
			return fmt.Errorf("initial file %q is invalid", f.Path)
		}
		if _, err := q.UpsertLabFile(ctx, dbgen.UpsertLabFileParams{SessionID: labID, Path: f.Path, Content: f.Content}); err != nil {
			return fmt.Errorf("seed file %s: %w", f.Path, err)
		}
	}
	return nil
}

// fileMap loads the lab's files for kubectl -f.
func fileMap(ctx context.Context, q *dbgen.Queries, labID uuid.UUID) (map[string]string, error) {
	rows, err := q.ListLabFiles(ctx, labID)
	if err != nil {
		return nil, fmt.Errorf("load files: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Path] = r.Content
	}
	return out, nil
}
