package maxbot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrContactNotVerified  = errors.New("MAX contact is not verified")
	ErrInvalidContactPhone = errors.New("MAX contact has invalid phone")
)

func ExtractVerifiedSharedContact(update Update, botToken string) (*SharedContact, error) {
	if update.UpdateType != "message_created" || update.Message == nil || update.Message.Body == nil {
		return nil, nil
	}
	for _, attachment := range update.Message.Body.Attachments {
		if attachment.Type != "contact" {
			continue
		}
		var payload ContactAttachmentPayload
		if err := json.Unmarshal(attachment.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode MAX contact attachment: %w", err)
		}
		if !VerifyContactHash(botToken, payload.VCFInfo, payload.Hash) {
			return nil, ErrContactNotVerified
		}
		phone, name := ParseVCF(payload.VCFInfo)
		normalizedPhone, err := NormalizeRussianPhone(phone)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(name) == "" {
			name = userDisplayName(update.Message.Sender)
		}
		return &SharedContact{Phone: normalizedPhone, Name: name}, nil
	}
	return nil, nil
}

func VerifyContactHash(botToken, vcfInfo, hash string) bool {
	botToken = strings.TrimSpace(botToken)
	hash = strings.TrimSpace(hash)
	if botToken == "" || strings.TrimSpace(vcfInfo) == "" || hash == "" {
		return false
	}

	vcfInfo = normalizeVCFLineBreaks(vcfInfo)
	mac := hmac.New(sha256.New, []byte(botToken))
	_, _ = mac.Write([]byte(vcfInfo))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(hash)), []byte(expected))
}

func ParseVCF(vcfInfo string) (phone, name string) {
	value := normalizeVCFLineBreaks(vcfInfo)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		index := strings.IndexByte(line, ':')
		if index < 0 {
			continue
		}
		property := strings.ToUpper(strings.TrimSpace(line[:index]))
		propertyValue := strings.TrimSpace(line[index+1:])
		switch {
		case (property == "TEL" || strings.HasPrefix(property, "TEL;")) && phone == "":
			phone = propertyValue
		case (property == "FN" || strings.HasPrefix(property, "FN;")) && name == "":
			name = propertyValue
		}
	}
	return phone, name
}

func NormalizeRussianPhone(value string) (string, error) {
	var digits strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}
	phone := digits.String()
	switch {
	case len(phone) == 10:
		phone = "7" + phone
	case len(phone) == 11 && phone[0] == '8':
		phone = "7" + phone[1:]
	}
	if len(phone) != 11 || phone[0] != '7' {
		return "", ErrInvalidContactPhone
	}
	return phone, nil
}

func normalizeVCFLineBreaks(value string) string {
	value = strings.ReplaceAll(value, `\r\n`, "\r\n")
	value = strings.ReplaceAll(value, `\n`, "\n")
	return value
}

func userDisplayName(user *User) string {
	if user == nil {
		return "Пользователь MAX"
	}
	parts := make([]string, 0, 2)
	if value := strings.TrimSpace(user.FirstName); value != "" {
		parts = append(parts, value)
	}
	if user.LastName != nil {
		if value := strings.TrimSpace(*user.LastName); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if user.Name != nil {
		if value := strings.TrimSpace(*user.Name); value != "" {
			return value
		}
	}
	if user.Username != nil {
		if value := strings.TrimSpace(*user.Username); value != "" {
			return value
		}
	}
	if user.UserID > 0 {
		return fmt.Sprintf("Пользователь MAX %d", user.UserID)
	}
	return "Пользователь MAX"
}
