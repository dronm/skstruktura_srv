package diadoc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

type ParsedDocument struct {
	Sender ParsedParty  `json:"sender"`
	Items  []ParsedItem `json:"items"`
	Totals ParsedTotals `json:"totals"`
}

type ParsedParty struct {
	Name string `json:"name"`
	INN  string `json:"inn"`
	KPP  string `json:"kpp"`
}

type ParsedItem struct {
	LineNum         int    `json:"line_num"`
	ProductCode     string `json:"product_code,omitempty"`
	Article         string `json:"article,omitempty"`
	GTIN            string `json:"gtin,omitempty"`
	Name            string `json:"name"`
	OKEI            string `json:"okei_code,omitempty"`
	UnitName        string `json:"unit_name,omitempty"`
	Quant           string `json:"quant"`
	Price           string `json:"price"`
	AmountWithoutVAT string `json:"amount_without_vat"`
	VATPercent      string `json:"vat_percent"`
	VATAmount       string `json:"vat_amount"`
	AmountWithVAT   string `json:"amount_with_vat"`
}

type ParsedTotals struct {
	AmountWithoutVAT string `json:"amount_without_vat"`
	VATAmount        string `json:"vat_amount"`
	AmountWithVAT    string `json:"amount_with_vat"`
}

type parsedItemWork struct {
	item  ParsedItem
	depth int
}

func ParseUTD(content []byte) (ParsedDocument, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = false
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(strings.TrimSpace(charset)) {
		case "windows-1251", "cp1251":
			return charmap.Windows1251.NewDecoder().Reader(input), nil
		case "utf-8", "utf8":
			return input, nil
		default:
			return nil, fmt.Errorf("unsupported UTD XML charset %q", charset)
		}
	}

	result := ParsedDocument{Items: make([]ParsedItem, 0)}
	stack := make([]string, 0, 16)
	sellerDepth := 0
	var current *parsedItemWork
	var text strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParsedDocument{}, fmt.Errorf("parse UTD XML: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			stack = append(stack, value.Name.Local)
			text.Reset()

			if isSellerElement(value.Name.Local) {
				sellerDepth = len(stack)
			}
			if sellerDepth > 0 {
				readSellerAttributes(&result.Sender, value)
			}

			if isItemElement(value.Name.Local) {
				item, parseErr := parsedItemFromAttributes(value)
				if parseErr != nil {
					return ParsedDocument{}, fmt.Errorf("parse UTD item %d: %w", len(result.Items)+1, parseErr)
				}
				current = &parsedItemWork{item: item, depth: len(stack)}
				continue
			}
			if current != nil {
				readItemAttributes(&current.item, value)
			}
			if isTotalsElement(value.Name.Local) {
				readTotalsAttributes(&result.Totals, value)
			}

		case xml.CharData:
			if current != nil {
				text.Write([]byte(value))
			}

		case xml.EndElement:
			if current != nil {
				readItemElementText(&current.item, value.Name.Local, text.String())
				if len(stack) == current.depth && isItemElement(value.Name.Local) {
					if err := normalizeParsedItem(&current.item, len(result.Items)+1); err != nil {
						return ParsedDocument{}, err
					}
					result.Items = append(result.Items, current.item)
					current = nil
				}
			}
			if sellerDepth > 0 && len(stack) == sellerDepth && isSellerElement(value.Name.Local) {
				sellerDepth = 0
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			text.Reset()
		}
	}

	if len(result.Items) == 0 {
		return ParsedDocument{}, fmt.Errorf("UTD does not contain material lines")
	}
	normalizeParsedTotals(&result)
	return result, nil
}

func isSellerElement(name string) bool {
	switch name {
	case "СвПрод", "СвПоставщик":
		return true
	default:
		return false
	}
}

func isItemElement(name string) bool {
	return name == "СведТов"
}

func isTotalsElement(name string) bool {
	return name == "ВсегоОпл" || name == "ВсегоОплПер"
}

func parsedItemFromAttributes(element xml.StartElement) (ParsedItem, error) {
	item := ParsedItem{
		Name:             attribute(element, "НаимТов", "НаимРаб", "НаимПредм"),
		OKEI:             attribute(element, "ОКЕИ_Тов", "ОКЕИ"),
		UnitName:         attribute(element, "НаимЕдИзм", "НаимЕд"),
		Quant:            attribute(element, "КолТов", "Колич"),
		Price:            attribute(element, "ЦенаТов", "Цена"),
		AmountWithoutVAT: attribute(element, "СтТовБезНДС", "СтБезНДС"),
		AmountWithVAT:    attribute(element, "СтТовУчНал", "СтУчНал"),
		ProductCode:      attribute(element, "КодТов", "Код"),
		Article:          attribute(element, "АртикулТов", "Артикул"),
		GTIN:             attribute(element, "ГТИН", "GTIN"),
	}
	if line := attribute(element, "НомСтр"); line != "" {
		if _, err := fmt.Sscan(line, &item.LineNum); err != nil {
			return ParsedItem{}, fmt.Errorf("invalid line number %q", line)
		}
	}
	return item, nil
}

func readSellerAttributes(sender *ParsedParty, element xml.StartElement) {
	if sender.Name == "" {
		sender.Name = attribute(element, "НаимОрг", "НаимОргИП", "НаимИП")
	}
	if sender.INN == "" {
		sender.INN = attribute(element, "ИННЮЛ", "ИННФЛ", "ИНН")
	}
	if sender.KPP == "" {
		sender.KPP = attribute(element, "КПП")
	}
}

func readItemAttributes(item *ParsedItem, element xml.StartElement) {
	if item.ProductCode == "" {
		item.ProductCode = attribute(element, "КодТов", "Код")
	}
	if item.Article == "" {
		item.Article = attribute(element, "АртикулТов", "Артикул")
	}
	if item.GTIN == "" {
		item.GTIN = attribute(element, "ГТИН", "GTIN")
	}
	if item.UnitName == "" {
		item.UnitName = attribute(element, "НаимЕдИзм", "НаимЕд")
	}
	if item.VATPercent == "" {
		item.VATPercent = attribute(element, "НалСт", "СтавкаНДС")
	}
	if item.VATAmount == "" {
		item.VATAmount = attribute(element, "СумНал", "СумНДС")
	}
}

func readItemElementText(item *ParsedItem, name string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	switch name {
	case "НалСт", "СтавкаНДС":
		item.VATPercent = value
	case "СумНал", "СумНДС":
		item.VATAmount = value
	case "СтТовУчНал", "СтУчНал":
		item.AmountWithVAT = value
	case "НаимЕдИзм", "НаимЕд":
		item.UnitName = value
	}
}

func readTotalsAttributes(totals *ParsedTotals, element xml.StartElement) {
	totals.AmountWithoutVAT = attribute(element, "СтТовБезНДСВсего", "СтБезНДСВсего")
	totals.VATAmount = attribute(element, "СумНалВсего", "СумНДСВсего")
	totals.AmountWithVAT = attribute(element, "СтТовУчНалВсего", "СтУчНалВсего")
}

func normalizeParsedItem(item *ParsedItem, fallbackLine int) error {
	if item.LineNum <= 0 {
		item.LineNum = fallbackLine
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		return fmt.Errorf("UTD item %d has no name", item.LineNum)
	}

	var err error
	item.Quant, err = normalizeDecimal(item.Quant, false)
	if err != nil || item.Quant == "0" {
		return fmt.Errorf("UTD item %d has invalid quantity %q", item.LineNum, item.Quant)
	}
	item.Price, err = normalizeDecimal(item.Price, true)
	if err != nil {
		return fmt.Errorf("UTD item %d has invalid price: %w", item.LineNum, err)
	}
	item.AmountWithoutVAT, err = normalizeDecimal(item.AmountWithoutVAT, true)
	if err != nil {
		return fmt.Errorf("UTD item %d has invalid amount without VAT: %w", item.LineNum, err)
	}
	item.VATPercent, err = normalizeVATPercent(item.VATPercent)
	if err != nil {
		return fmt.Errorf("UTD item %d has invalid VAT rate: %w", item.LineNum, err)
	}
	if decimalRat(item.VATPercent).Cmp(big.NewRat(100, 1)) > 0 {
		return fmt.Errorf("UTD item %d VAT rate exceeds 100 percent", item.LineNum)
	}
	item.VATAmount, err = normalizeDecimal(item.VATAmount, true)
	if err != nil {
		return fmt.Errorf("UTD item %d has invalid VAT amount: %w", item.LineNum, err)
	}
	item.AmountWithVAT, err = normalizeDecimal(item.AmountWithVAT, true)
	if err != nil {
		return fmt.Errorf("UTD item %d has invalid amount with VAT: %w", item.LineNum, err)
	}

	if item.AmountWithVAT == "0" {
		item.AmountWithVAT = addDecimals(item.AmountWithoutVAT, item.VATAmount, 2)
	}
	if item.AmountWithoutVAT == "0" && item.AmountWithVAT != "0" {
		item.AmountWithoutVAT = subtractDecimals(item.AmountWithVAT, item.VATAmount, 2)
	}
	if item.VATAmount == "0" && item.VATPercent != "0" && item.AmountWithVAT != "0" {
		item.VATAmount = includedVAT(item.AmountWithVAT, item.VATPercent)
		item.AmountWithoutVAT = subtractDecimals(item.AmountWithVAT, item.VATAmount, 2)
	}
	if decimalRat(item.VATAmount).Cmp(decimalRat(item.AmountWithVAT)) > 0 {
		return fmt.Errorf("UTD item %d VAT amount exceeds gross amount", item.LineNum)
	}

	item.ProductCode = normalizeMatchKey(item.ProductCode)
	item.Article = normalizeMatchKey(item.Article)
	item.GTIN = normalizeMatchKey(item.GTIN)
	item.OKEI = strings.TrimSpace(item.OKEI)
	item.UnitName = strings.TrimSpace(item.UnitName)
	return nil
}

func normalizeParsedTotals(document *ParsedDocument) {
	withoutVAT, vat, withVAT := new(big.Rat), new(big.Rat), new(big.Rat)
	for _, item := range document.Items {
		withoutVAT.Add(withoutVAT, decimalRat(item.AmountWithoutVAT))
		vat.Add(vat, decimalRat(item.VATAmount))
		withVAT.Add(withVAT, decimalRat(item.AmountWithVAT))
	}
	document.Totals.AmountWithoutVAT = firstNonZeroDecimal(document.Totals.AmountWithoutVAT, ratDecimal(withoutVAT, 2))
	document.Totals.VATAmount = firstNonZeroDecimal(document.Totals.VATAmount, ratDecimal(vat, 2))
	document.Totals.AmountWithVAT = firstNonZeroDecimal(document.Totals.AmountWithVAT, ratDecimal(withVAT, 2))
}

func attribute(element xml.StartElement, names ...string) string {
	for _, name := range names {
		for _, attr := range element.Attr {
			if attr.Name.Local == name {
				return strings.TrimSpace(attr.Value)
			}
		}
	}
	return ""
}

func normalizeDecimal(value string, allowZero bool) (string, error) {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\u00a0' {
			return -1
		}
		if r == ',' {
			return '.'
		}
		return r
	}, strings.TrimSpace(value))
	if value == "" || value == "-" || strings.Contains(strings.ToLower(value), "безндс") {
		return "0", nil
	}
	result, ok := new(big.Rat).SetString(value)
	if !ok || result.Sign() < 0 || (!allowZero && result.Sign() == 0) {
		return "", fmt.Errorf("invalid non-negative decimal %q", value)
	}
	if strings.Contains(value, ".") {
		value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	}
	if value == "" {
		value = "0"
	}
	return value, nil
}

func normalizeVATPercent(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || normalized == "-" || strings.Contains(normalized, "без ндс") {
		return "0", nil
	}
	normalized = strings.TrimSpace(strings.TrimSuffix(normalized, "%"))
	return normalizeDecimal(normalized, true)
}

func normalizeMatchKey(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func firstNonZeroDecimal(value string, fallback string) string {
	value, err := normalizeDecimal(value, true)
	if err == nil && value != "0" {
		return value
	}
	return fallback
}

func decimalRat(value string) *big.Rat {
	result, ok := new(big.Rat).SetString(value)
	if !ok {
		return new(big.Rat)
	}
	return result
}

func addDecimals(left string, right string, scale int) string {
	result := new(big.Rat).Add(decimalRat(left), decimalRat(right))
	return ratDecimal(result, scale)
}

func subtractDecimals(left string, right string, scale int) string {
	result := new(big.Rat).Sub(decimalRat(left), decimalRat(right))
	if result.Sign() < 0 {
		return "0"
	}
	return ratDecimal(result, scale)
}

func includedVAT(gross string, percent string) string {
	rate := decimalRat(percent)
	denominator := new(big.Rat).Add(rate, big.NewRat(100, 1))
	if denominator.Sign() == 0 {
		return "0"
	}
	result := new(big.Rat).Mul(decimalRat(gross), rate)
	result.Quo(result, denominator)
	return ratDecimal(result, 2)
}

func ratDecimal(value *big.Rat, scale int) string {
	if value == nil {
		return "0"
	}
	return value.FloatString(scale)
}
