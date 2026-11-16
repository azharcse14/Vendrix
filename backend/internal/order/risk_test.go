package order

import (
	"slices"
	"testing"
)

var limits = RiskLimits{CODAmountOver: 300000, NewPhoneAmountOver: 200000, MaxOrdersPerHour: 3} // 9.2 default

// 8.7, 8.19 item 3: protiti flag er shima (`>`, shoman na; many_orders ager order >= max - 1).
func TestRiskFlags(t *testing.T) {
	old := RiskHistory{PriorOrders: true} // purono phone, kono jhuki nai
	cases := []struct {
		name  string
		h     RiskHistory
		cod   bool
		total int64
		want  []string
	}{
		{"kichu na", old, true, 300000, nil},
		{"cod shima er beshi", old, true, 300001, []string{"cod_high_amount"}},
		{"online e cod_high_amount na", old, false, 900000, nil},
		{"returned", RiskHistory{PriorOrders: true, Returned: true}, false, 100, []string{"phone_refused_before"}},
		{"ghontay 2 ta ager order = ei ta 3 tomo", RiskHistory{PriorOrders: true, RecentOrders: 2}, true, 100, []string{"many_orders"}},
		{"ghontay 1 ta ager order", RiskHistory{PriorOrders: true, RecentOrders: 1}, true, 100, nil},
		{"notun phone, shima", RiskHistory{}, true, 200000, nil},
		{"notun phone, shima er beshi", RiskHistory{}, false, 200001, []string{"new_phone_high_amount"}},
		{"shob", RiskHistory{Returned: true, RecentOrders: 5}, true, 300001,
			[]string{"cod_high_amount", "phone_refused_before", "many_orders", "new_phone_high_amount"}},
	}
	for _, c := range cases {
		if got := RiskFlags(c.h, limits, c.cod, c.total); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if got := RiskFlags(RiskHistory{PriorOrders: true}, RiskLimits{MaxOrdersPerHour: 1, CODAmountOver: 1 << 40, NewPhoneAmountOver: 1 << 40}, true, 1); !slices.Equal(got, []string{"many_orders"}) {
		t.Errorf("max 1: prothom order-i many_orders, got %v", got)
	}
}

// 8.7 otp_mode table; 8.19 item 4: login customer e kokhono OTP na.
func TestOTPRequired(t *testing.T) {
	flagged := []string{"many_orders"}
	for _, mode := range []string{"never", "risky", "all_cod", "all", "bhul"} {
		for _, flags := range [][]string{nil, flagged} {
			for _, cod := range []bool{true, false} {
				for _, login := range []bool{false, true} {
					var want bool
					switch mode {
					case "risky":
						want = len(flags) > 0 && cod
					case "all_cod":
						want = cod
					case "all", "bhul": // ochena mode e OTP (fail closed)
						want = true
					}
					want = want && !login
					if got := OTPRequired(mode, flags, cod, login); got != want {
						t.Errorf("mode %s, flags %v, cod %v, login %v: got %v want %v", mode, flags, cod, login, got, want)
					}
				}
			}
		}
	}
}

// 8.7, 8.19 item 4: cod_confirmation; risky = flag ar otp_verified false; online e kokhono na.
func TestConfirmRequired(t *testing.T) {
	flagged := []string{"cod_high_amount"}
	for _, mode := range []string{"none", "risky", "all", "bhul"} {
		for _, flags := range [][]string{nil, flagged} {
			for _, cod := range []bool{true, false} {
				for _, verified := range []bool{false, true} {
					var want bool
					switch mode {
					case "risky":
						want = len(flags) > 0 && !verified
					case "all", "bhul": // ochena mode e confirm (fail closed)
						want = true
					}
					want = want && cod
					if got := ConfirmRequired(mode, flags, cod, verified); got != want {
						t.Errorf("mode %s, flags %v, cod %v, otp_verified %v: got %v want %v", mode, flags, cod, verified, got, want)
					}
				}
			}
		}
	}
}
