package diadoc

import "testing"

func TestParseUTD(t *testing.T) {
	t.Parallel()

	content := []byte(`<?xml version="1.0" encoding="utf-8"?>
<Файл>
	<Документ>
		<СвСчФакт>
			<СвПрод><ИдСв><СвЮЛУч НаимОрг="ООО Поставщик" ИННЮЛ="1234567890" КПП="123456789"/></ИдСв></СвПрод>
		</СвСчФакт>
		<ТаблСчФакт>
			<СведТов НомСтр="1" НаимТов="Цемент М500" ОКЕИ_Тов="796" КолТов="2" ЦенаТов="5000" СтТовБезНДС="10000">
				<НалСт>20%</НалСт>
				<СумНал><СумНал>2000</СумНал></СумНал>
				<СтТовУчНал>12000</СтТовУчНал>
				<ДопСведТов КодТов="CM500" АртикулТов="A-42"/>
			</СведТов>
			<ВсегоОпл СтТовБезНДСВсего="10000" СумНалВсего="2000" СтТовУчНалВсего="12000"/>
		</ТаблСчФакт>
	</Документ>
</Файл>`)

	result, err := ParseUTD(content)
	if err != nil {
		t.Fatalf("ParseUTD() error = %v", err)
	}
	if result.Sender.INN != "1234567890" || result.Sender.KPP != "123456789" {
		t.Fatalf("sender = %#v", result.Sender)
	}
	if len(result.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.ProductCode != "CM500" || item.Article != "A-42" {
		t.Fatalf("item keys = %#v", item)
	}
	if item.VATPercent != "20" || item.VATAmount != "2000" || item.AmountWithVAT != "12000" {
		t.Fatalf("item VAT = %#v", item)
	}
	if result.Totals.AmountWithVAT != "12000" {
		t.Fatalf("totals = %#v", result.Totals)
	}
}

func TestParseUTDCalculatesIncludedVAT(t *testing.T) {
	t.Parallel()

	content := []byte(`<Файл><Документ><ТаблСчФакт>
		<СведТов НомСтр="1" НаимТов="Материал" КолТов="1" ЦенаТов="100" СтТовУчНал="120">
			<НалСт>20%</НалСт>
		</СведТов>
	</ТаблСчФакт></Документ></Файл>`)

	result, err := ParseUTD(content)
	if err != nil {
		t.Fatalf("ParseUTD() error = %v", err)
	}
	item := result.Items[0]
	if item.VATAmount != "20.00" || item.AmountWithoutVAT != "100.00" {
		t.Fatalf("calculated item = %#v", item)
	}
}

func TestParseUTDIgnoresTraceabilitySubelements(t *testing.T) {
	t.Parallel()

	content := []byte(`<Файл><Документ><ТаблСчФакт>
		<СведТов НомСтр="1" НаимТов="Материал" КолТов="1" СтТовУчНал="120">
			<НалСт>20%</НалСт>
			<СведПрослеж НомПартии="ABC"/>
		</СведТов>
	</ТаблСчФакт></Документ></Файл>`)

	result, err := ParseUTD(content)
	if err != nil {
		t.Fatalf("ParseUTD() error = %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(result.Items))
	}
}
