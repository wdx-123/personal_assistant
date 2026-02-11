package util

// DesensitizePhone 手机号脱敏处理 (保留前3后4，中间4位掩码)
func DesensitizePhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
