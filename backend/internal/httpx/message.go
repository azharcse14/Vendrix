package httpx

import (
	"net/http"
	"strings"
)

// messages (7.1, v1.10): error code → {bn, en}. Client `code` diye logic kore; notun code ekhane jog koro.
var messages = map[string][2]string{
	"validation_failed":        {"তথ্যে ভুল আছে", "Some information is invalid"},
	"not_found":                {"খুঁজে পাওয়া যায়নি", "Not found"},
	"method_not_allowed":       {"এই অনুরোধ চলবে না", "Method not allowed"},
	"unauthenticated":          {"আবার লগইন করুন", "Please log in"},
	"forbidden":                {"এই কাজের অনুমতি নেই", "You are not allowed to do this"},
	"shop_not_found":           {"দোকান পাওয়া যায়নি", "Shop not found"},
	"shop_suspended":           {"দোকান এখন বন্ধ", "This shop is closed for now"},
	"slug_taken":               {"এই লিংক আগেই ব্যবহার হয়েছে", "This link is already used"},
	"category_not_empty":       {"এই ক্যাটাগরিতে পণ্য বা উপ-ক্যাটাগরি আছে", "This category is not empty"},
	"sku_taken":                {"এই SKU আগেই ব্যবহার হয়েছে", "This SKU is already used"},
	"invalid_phone":            {"মোবাইল নম্বর ঠিক নেই", "Invalid mobile number"},
	"turnstile_failed":         {"যাচাই হয়নি, আবার চেষ্টা করুন", "Verification failed, please try again"},
	"phone_blocked":            {"এই নম্বরে লগইন বন্ধ", "Login is blocked for this number"},
	"otp_invalid":              {"কোডটি ঠিক নেই বা মেয়াদ শেষ", "The code is wrong or expired"},
	"otp_too_many_attempts":    {"অনেকবার ভুল হয়েছে, নতুন কোড নিন", "Too many wrong tries, request a new code"},
	"invalid_credentials":      {"ইমেইল বা পাসওয়ার্ড ঠিক নেই", "Wrong email or password"},
	"mfa_invalid":              {"কোডটি ঠিক নেই", "The code is wrong"},
	"mfa_too_many_attempts":    {"অনেকবার ভুল হয়েছে, আবার লগইন করুন", "Too many wrong tries, log in again"},
	"setup_required":           {"আগে পাসওয়ার্ড আর দুই ধাপের যাচাই সেট করুন", "Finish password and two-step setup first"},
	"password_change_required": {"আগে পাসওয়ার্ড বদলান", "Change your password first"},
	"session_expired":          {"সেশনের মেয়াদ শেষ, আবার লগইন করুন", "Session expired, please log in again"},
	"rate_limited":             {"অনেক বেশি অনুরোধ, একটু পরে চেষ্টা করুন", "Too many requests, try again later"},
	"db_unavailable":           {"সার্ভারে সমস্যা, একটু পরে চেষ্টা করুন", "Service unavailable, try again later"},
	"internal":                 {"সার্ভারে সমস্যা হয়েছে", "Something went wrong"},
}

// lang: Accept-Language er prothom bhasha en hole 1 (en), na hole 0 (bn, fallback).
// ponytail: q-value dekhe na, prothom tag-i; browser prothome pochonder bhasha-i pathay.
func lang(r *http.Request) int {
	first, _, _ := strings.Cut(r.Header.Get("Accept-Language"), ",")
	first, _, _ = strings.Cut(strings.TrimSpace(first), ";")
	if tag, _, _ := strings.Cut(strings.ToLower(first), "-"); tag == "en" {
		return 1
	}
	return 0
}

// Locale (7.1): public API er lekha kon bhasha e: "en" ba "bn" (fallback).
func Locale(r *http.Request) string {
	return [2]string{"bn", "en"}[lang(r)]
}

func message(r *http.Request, code string) string {
	m, ok := messages[code]
	if !ok {
		m = messages["internal"]
	}
	return m[lang(r)]
}
