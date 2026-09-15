package catalog

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrReadingPreprocess marks a failure to build the immutable reading
// artifacts the reader backend serves.
var ErrReadingPreprocess = errors.New("prepare reading content")

// ReadingPreparer builds the reading artifacts for one stored original.
// The reader backend serves nothing but these artifacts, so a book published
// without them answers 409 reading content is not ready.
type ReadingPreparer interface {
	Prepare(ctx context.Context, bookKey, sourcePath string) error
}

// DefaultPreprocessTimeout bounds one reading artifact build. Large books take
// tens of seconds; the bound only exists to keep a wedged tool from blocking
// an import worker slot forever.
const DefaultPreprocessTimeout = 10 * time.Minute

// PreprocessCommand runs the reader_content_preprocess tool. The tool stages
// into a temp directory next to the destination and only publishes on success,
// so a failed run leaves any previous `current` version untouched. Re-running
// it for an already built content version is a no-op.
type PreprocessCommand struct {
	binary     string
	outputRoot string
}

func NewPreprocessCommand(binary, outputRoot string) *PreprocessCommand {
	return &PreprocessCommand{binary: binary, outputRoot: outputRoot}
}

func (p *PreprocessCommand) Prepare(ctx context.Context, bookKey, sourcePath string) error {
	runCtx, cancel := context.WithTimeout(ctx, DefaultPreprocessTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, p.binary,
		"--source", sourcePath,
		"--output", p.outputRoot,
		"--book-id", bookKey,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		diagnostic := strings.TrimSpace(string(output))
		if diagnostic == "" {
			return fmt.Errorf("%w: %v: %v", ErrReadingPreprocess, p.binary, err)
		}
		return fmt.Errorf("%w: %v: %v: %s", ErrReadingPreprocess, p.binary, err, diagnostic)
	}
	return nil
}
