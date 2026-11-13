package phone

import "strings"

/**
 * NOTE: Canonical turns a digits-only international number into the form the app stores. WhatsApp still registers
 * many Brazilian mobiles without the ninth digit (55 11 8888-7777 instead of 55 11 98888-7777), so a 12-digit
 * Brazilian number whose local part starts with 6 to 9 (a mobile) gets the 9 back. Landlines stay as they are.
 */
func Canonical(digits string) string {
	if len(digits) == 12 && strings.HasPrefix(digits, "55") && digits[4] >= '6' && digits[4] <= '9' {
		return digits[:4] + "9" + digits[4:]
	}
	return digits
}
