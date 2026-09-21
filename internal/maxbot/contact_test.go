package maxbot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeRussianPhone(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "+7(922)-269-52-51", want: "79222695251"},
		{input: "8 922 269 52 51", want: "79222695251"},
		{input: "9222695251", want: "79222695251"},
		{input: "79222695251", want: "79222695251"},
	}
	for _, test := range tests {
		got, err := NormalizeRussianPhone(test.input)
		if err != nil {
			t.Fatalf("NormalizeRussianPhone(%q): %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("NormalizeRussianPhone(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestNormalizeRussianPhoneRejectsInvalidNumber(t *testing.T) {
	if _, err := NormalizeRussianPhone("12345"); !errors.Is(err, ErrInvalidContactPhone) {
		t.Fatalf("NormalizeRussianPhone() error = %v, want ErrInvalidContactPhone", err)
	}
}

func TestParseVCF(t *testing.T) {
	vcf := "BEGIN:VCARD\r\nVERSION:3.0\r\nTEL;TYPE=cell:+7 (922) 269-52-51\r\nFN:Ivan Ivanov\r\nEND:VCARD\r\n"
	phone, name := ParseVCF(vcf)
	if phone != "+7 (922) 269-52-51" {
		t.Fatalf("phone = %q", phone)
	}
	if name != "Ivan Ivanov" {
		t.Fatalf("name = %q", name)
	}
}

func TestVerifyContactHash(t *testing.T) {
	const token = "secret-token"
	vcf := "BEGIN:VCARD\r\nVERSION:3.0\r\nTEL;TYPE=cell:79222695251\r\nFN:Ivan Ivanov\r\nEND:VCARD\r\n"
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(vcf))
	hash := hex.EncodeToString(mac.Sum(nil))
	if !VerifyContactHash(token, vcf, hash) {
		t.Fatal("VerifyContactHash() = false")
	}
	if VerifyContactHash(token, vcf, "bad-hash") {
		t.Fatal("VerifyContactHash() accepted invalid hash")
	}
}

func TestExtractVerifiedSharedContact(t *testing.T) {
	const token = "secret-token"
	vcf := "BEGIN:VCARD\r\nVERSION:3.0\r\nTEL;TYPE=cell:79222695251\r\nFN:Ivan Ivanov\r\nEND:VCARD\r\n"
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(vcf))
	payload, err := json.Marshal(ContactAttachmentPayload{
		VCFInfo: vcf,
		Hash:    hex.EncodeToString(mac.Sum(nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	update := Update{
		UpdateType: "message_created",
		Message: &Message{
			Sender: &User{UserID: 123},
			Body: &MessageBody{Attachments: []Attachment{
				{Type: "contact", Payload: payload},
			}},
		},
	}
	contact, err := ExtractVerifiedSharedContact(update, token)
	if err != nil {
		t.Fatalf("ExtractVerifiedSharedContact(): %v", err)
	}
	if contact == nil {
		t.Fatal("contact is nil")
	}
	if contact.Phone != "79222695251" || contact.Name != "Ivan Ivanov" {
		t.Fatalf("contact = %#v", contact)
	}
}
