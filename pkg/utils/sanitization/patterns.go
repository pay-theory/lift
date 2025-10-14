package sanitization

import "regexp"

// PaymentXMLPatterns contains pre-configured patterns for common payment processing XML elements
// These patterns handle both regular XML and HTML-escaped XML for maximum compatibility
//
// Usage:
//
//	sanitized := sanitization.SanitizeXML(xmlString, sanitization.PaymentXMLPatterns)
//
// Patterns are applied in order, so more specific patterns should come before general ones.
var PaymentXMLPatterns = []XMLSanitizationPattern{
	// Card numbers (AcctNum) - show last 4 digits only
	{
		Name:        "AcctNum",
		Pattern:     regexp.MustCompile(`<AcctNum>(\d{12,19})</AcctNum>`),
		MaskingFunc: MaskCardNumber,
	},
	{
		Name:        "AcctNum_Escaped",
		Pattern:     regexp.MustCompile(`&lt;AcctNum&gt;(\d{12,19})&lt;/AcctNum&gt;`),
		MaskingFunc: MaskCardNumber,
	},

	// Card expiry dates - mask completely (PCI requirement)
	{
		Name:        "CardExpiryDate",
		Pattern:     regexp.MustCompile(`<CardExpiryDate>(\d+)</CardExpiryDate>`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},
	{
		Name:        "CardExpiryDate_Escaped",
		Pattern:     regexp.MustCompile(`&lt;CardExpiryDate&gt;(\d+)&lt;/CardExpiryDate&gt;`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},

	// CVV/CCV data - mask completely (never log CVV)
	{
		Name:        "CCVData",
		Pattern:     regexp.MustCompile(`<CCVData>(\d+)</CCVData>`),
		MaskingFunc: MaskCompletelyFunc("***"),
	},
	{
		Name:        "CCVData_Escaped",
		Pattern:     regexp.MustCompile(`&lt;CCVData&gt;(\d+)&lt;/CCVData&gt;`),
		MaskingFunc: MaskCompletelyFunc("***"),
	},

	// CVV (alternative field name)
	{
		Name:        "CVV",
		Pattern:     regexp.MustCompile(`<CVV>(\d+)</CVV>`),
		MaskingFunc: MaskCompletelyFunc("***"),
	},
	{
		Name:        "CVV_Escaped",
		Pattern:     regexp.MustCompile(`&lt;CVV&gt;(\d+)&lt;/CVV&gt;`),
		MaskingFunc: MaskCompletelyFunc("***"),
	},

	// SecurityCode (alternative field name)
	{
		Name:        "SecurityCode",
		Pattern:     regexp.MustCompile(`<SecurityCode>(\d+)</SecurityCode>`),
		MaskingFunc: MaskCompletelyFunc("***"),
	},
	{
		Name:        "SecurityCode_Escaped",
		Pattern:     regexp.MustCompile(`&lt;SecurityCode&gt;(\d+)&lt;/SecurityCode&gt;`),
		MaskingFunc: MaskCompletelyFunc("***"),
	},

	// TransArmorToken - show last 4 characters
	{
		Name:        "TransArmorToken",
		Pattern:     regexp.MustCompile(`<TransArmorToken>([^<]+)</TransArmorToken>`),
		MaskingFunc: MaskTokenLastFour,
	},
	{
		Name:        "TransArmorToken_Escaped",
		Pattern:     regexp.MustCompile(`&lt;TransArmorToken&gt;([^<]+)&lt;/TransArmorToken&gt;`),
		MaskingFunc: MaskTokenLastFour,
	},

	// SSN - mask completely
	{
		Name:        "SSN",
		Pattern:     regexp.MustCompile(`<SSN>([^<]+)</SSN>`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},
	{
		Name:        "SSN_Escaped",
		Pattern:     regexp.MustCompile(`&lt;SSN&gt;([^<]+)&lt;/SSN&gt;`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},

	// TaxID - mask completely
	{
		Name:        "TaxID",
		Pattern:     regexp.MustCompile(`<TaxID>([^<]+)</TaxID>`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},
	{
		Name:        "TaxID_Escaped",
		Pattern:     regexp.MustCompile(`&lt;TaxID&gt;([^<]+)&lt;/TaxID&gt;`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},

	// TaxId (alternative capitalization)
	{
		Name:        "TaxId",
		Pattern:     regexp.MustCompile(`<TaxId>([^<]+)</TaxId>`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},
	{
		Name:        "TaxId_Escaped",
		Pattern:     regexp.MustCompile(`&lt;TaxId&gt;([^<]+)&lt;/TaxId&gt;`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},

	// PIN - mask completely
	{
		Name:        "PIN",
		Pattern:     regexp.MustCompile(`<PIN>([^<]+)</PIN>`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},
	{
		Name:        "PIN_Escaped",
		Pattern:     regexp.MustCompile(`&lt;PIN&gt;([^<]+)&lt;/PIN&gt;`),
		MaskingFunc: MaskCompletelyFunc("****"),
	},

	// Account number (for ACH) - show last 4 digits
	{
		Name:        "AccountNumber",
		Pattern:     regexp.MustCompile(`<AccountNumber>(\d{4,17})</AccountNumber>`),
		MaskingFunc: MaskCardNumber,
	},
	{
		Name:        "AccountNumber_Escaped",
		Pattern:     regexp.MustCompile(`&lt;AccountNumber&gt;(\d{4,17})&lt;/AccountNumber&gt;`),
		MaskingFunc: MaskCardNumber,
	},

	// Routing number (for ACH) - show last 4 digits
	{
		Name:        "RoutingNumber",
		Pattern:     regexp.MustCompile(`<RoutingNumber>(\d{9})</RoutingNumber>`),
		MaskingFunc: MaskCardNumber,
	},
	{
		Name:        "RoutingNumber_Escaped",
		Pattern:     regexp.MustCompile(`&lt;RoutingNumber&gt;(\d{9})&lt;/RoutingNumber&gt;`),
		MaskingFunc: MaskCardNumber,
	},
}

// RapidConnectXMLPatterns is an alias for PaymentXMLPatterns for backward compatibility
// and explicit use with Rapid Connect (FiServ) integrations
var RapidConnectXMLPatterns = PaymentXMLPatterns
