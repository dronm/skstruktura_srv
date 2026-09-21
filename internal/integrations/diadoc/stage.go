package diadoc

import (
	"strings"
	"time"
)

func stageInputFromEvent(
	boxID string,
	event BoxEvent,
	message Message,
	entity Entity,
) StageDocumentInput {
	input := StageDocumentInput{
		BoxID:          boxID,
		MessageID:      message.MessageID,
		EntityID:       entity.EntityID,
		ParentEntityID: entity.ParentEntityID,
		EventID:        event.EventID,
		EventIndexKey:  event.IndexKey,
		EventTimestamp: timeFromDotNetTicks(message.TimestampTicks),
		AttachmentType: entity.AttachmentType,
		FileName:       entity.FileName,
		SenderBoxID:    message.FromBoxID,
		SenderName:     message.FromTitle,
	}
	if entity.DocumentInfo == nil {
		return input
	}
	info := entity.DocumentInfo
	input.TypeNamedID = info.TypeNamedID
	input.DocumentFunction = info.Function
	input.DocumentVersion = info.Version
	input.DocumentNumber = info.DocumentNumber
	input.SenderBoxID = firstNonEmpty(info.CounteragentBoxID, input.SenderBoxID)
	if documentDate, ok := parseDiadocDate(info.DocumentDate); ok {
		input.DocumentDate = &documentDate
	}
	return input
}

func parseDiadocDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		"2006-01-02",
		"02.01.2006",
		"02/01/2006",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		result, err := time.Parse(layout, value)
		if err == nil {
			return result, true
		}
	}
	return time.Time{}, false
}

func timeFromDotNetTicks(ticks int64) time.Time {
	if ticks <= dotNetUnixEpochTicks {
		return time.Time{}
	}
	unixTicks := ticks - dotNetUnixEpochTicks
	seconds := unixTicks / 10_000_000
	nanoseconds := (unixTicks % 10_000_000) * 100
	return time.Unix(seconds, nanoseconds).UTC()
}

func applyStageResult(result *SyncResult, staged StageDocumentResult) {
	if result == nil {
		return
	}
	if staged.Added {
		result.DocumentsAdded++
	}
	if staged.Updated {
		result.DocumentsUpdated++
	}
	if staged.Failed {
		result.DocumentsFailed++
	}
}
