package validator

import (
	"regexp"
	"strings"
)

var gstinRegex = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z]{1}[1-9A-Z]{1}Z[0-9A-Z]{1}$`)

var stateCodes = map[string]string{
	"01": "Jammu & Kashmir",
	"02": "Himachal Pradesh",
	"03": "Punjab",
	"04": "Chandigarh",
	"05": "Uttarakhand",
	"06": "Haryana",
	"07": "Delhi",
	"08": "Rajasthan",
	"09": "Uttar Pradesh",
	"10": "Bihar",
	"11": "Sikkim",
	"12": "Arunachal Pradesh",
	"13": "Nagaland",
	"14": "Manipur",
	"15": "Mizoram",
	"16": "Tripura",
	"17": "Meghalaya",
	"18": "Assam",
	"19": "West Bengal",
	"20": "Jharkhand",
	"21": "Odisha",
	"22": "Chhattisgarh",
	"23": "Madhya Pradesh",
	"24": "Gujarat",
	"26": "Dadra & Nagar Haveli and Daman & Diu",
	"27": "Maharashtra",
	"28": "Andhra Pradesh",
	"29": "Karnataka",
	"30": "Goa",
	"31": "Lakshadweep",
	"32": "Kerala",
	"33": "Tamil Nadu",
	"34": "Puducherry",
	"35": "Andaman & Nicobar Islands",
	"36": "Telangana",
	"37": "Andhra Pradesh (new)",
	"38": "Ladakh",
}

// ValidateGSTIN validates Indian GSTIN format and checksum.
func ValidateGSTIN(gstin string) bool {
	gstin = strings.ToUpper(strings.TrimSpace(gstin))
	if len(gstin) != 15 {
		return false
	}

	// 1. Regex check
	if !gstinRegex.MatchString(gstin) {
		return false
	}

	// 2. State code check
	stateCode := gstin[:2]
	if _, ok := stateCodes[stateCode]; !ok {
		return false
	}

	// 3. Checksum validation
	return validateChecksum(gstin)
}

// ValidateGSTINWithStateCode validates GSTIN and also checks that the state code matches the expected state.
func ValidateGSTINWithStateCode(gstin, expectedStateCode string) bool {
	if !ValidateGSTIN(gstin) {
		return false
	}
	return gstin[:2] == expectedStateCode
}

// GetStateFromGSTIN extracts the state code from GSTIN.
func GetStateFromGSTIN(gstin string) string {
	if len(gstin) < 2 {
		return ""
	}
	return gstin[:2]
}

// GetStateNameFromGSTIN returns the full state name from GSTIN state code.
func GetStateNameFromGSTIN(gstin string) string {
	code := GetStateFromGSTIN(gstin)
	return stateCodes[code]
}

// IsInterstateTx returns true if store and customer are in different states.
// Used to determine IGST vs CGST+SGST.
func IsInterstateTx(storeGSTIN, customerGSTIN string) bool {
	if storeGSTIN == "" || customerGSTIN == "" {
		return false // B2C or missing info is usually considered intrastate or handled elsewhere
	}
	storeState := GetStateFromGSTIN(storeGSTIN)
	customerState := GetStateFromGSTIN(customerGSTIN)
	return storeState != customerState
}

// validateChecksum implements the official GSTN checksum algorithm (Weighted Sum Modulo 36).
func validateChecksum(gstin string) bool {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	
	charToVal := make(map[rune]int)
	for i, r := range chars {
		charToVal[r] = i
	}

	sum := 0
	for i := 0; i < 14; i++ {
		val := charToVal[rune(gstin[i])]
		weight := 1
		if i%2 != 0 {
			weight = 2
		}

		product := val * weight
		factor := (product / 36) + (product % 36)
		sum += factor
	}

	rem := sum % 36
	checkDigitVal := (36 - rem) % 36
	expectedCheckDigit := chars[checkDigitVal]

	return gstin[14] == expectedCheckDigit
}
