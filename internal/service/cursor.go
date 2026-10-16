package service

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

const cursorSeparator = "|"

var errMalformedCursor = errors.New("malformed cursor")

func encodeActivityCursor(c model.ActivityCursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339) + cursorSeparator + c.EntryID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeActivityCursor(token string) (model.ActivityCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return model.ActivityCursor{}, err
	}

	createdAt, entryID, ok := strings.Cut(string(raw), cursorSeparator)
	if !ok {
		return model.ActivityCursor{}, errMalformedCursor
	}

	at, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return model.ActivityCursor{}, err
	}
	id, err := uuid.Parse(entryID)
	if err != nil {
		return model.ActivityCursor{}, err
	}

	return model.ActivityCursor{CreatedAt: at, EntryID: id}, nil
}
