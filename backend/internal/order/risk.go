package order

// RiskLimits: settings er checkout.risk (9.2).
type RiskLimits struct {
	CODAmountOver      int64 `json:"cod_amount_over"`
	NewPhoneAmountOver int64 `json:"new_phone_amount_over"`
	MaxOrdersPerHour   int   `json:"max_orders_per_hour"`
}

// RiskHistory: ei shop e ager order theke (LoadRiskHistory).
type RiskHistory struct {
	RecentOrders int  // shesh 60 minute e ekoi customer_phone ba ip, shob status
	Returned     bool // customer ba recipient phone er ager order er fulfillment returned
	PriorOrders  bool // customer_phone er ager order ache (cancelled o)
}

// RiskFlags (8.7, 8.19 item 3): total = expected_total; kram 8.7 er table er, orders.risk_flags e jay.
func RiskFlags(h RiskHistory, l RiskLimits, cod bool, total int64) []string {
	var flags []string
	if cod && total > l.CODAmountOver {
		flags = append(flags, "cod_high_amount")
	}
	if h.Returned {
		flags = append(flags, "phone_refused_before")
	}
	if h.RecentOrders >= l.MaxOrdersPerHour-1 { // ei order shoho max
		flags = append(flags, "many_orders")
	}
	if !h.PriorOrders && total > l.NewPhoneAmountOver {
		flags = append(flags, "new_phone_high_amount")
	}
	return flags
}

// OTPRequired (8.7 otp_mode, 8.19 item 4): login customer er phone already OTP e proman. Ochena mode e
// OTP, karon bhul setting e jhuki newa na.
func OTPRequired(mode string, flags []string, cod, loggedIn bool) bool {
	if loggedIn {
		return false
	}
	switch mode {
	case "never":
		return false
	case "risky":
		return cod && len(flags) > 0
	case "all_cod":
		return cod
	}
	return true
}

// ConfirmRequired (8.7 cod_confirmation, 8.19 item 4): shudhu COD; risky e OTP e phone proman hole SMS
// confirm lage na. Ochena mode e confirm (fail closed).
func ConfirmRequired(mode string, flags []string, cod, otpVerified bool) bool {
	if !cod {
		return false
	}
	switch mode {
	case "none":
		return false
	case "risky":
		return len(flags) > 0 && !otpVerified
	}
	return true
}
