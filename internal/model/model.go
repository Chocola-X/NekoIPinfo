package model

type IPInfo struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	ISP       string `json:"isp"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type IPRule struct {
	NetworkStart uint32
	NetworkEnd   uint32
	Info         IPInfo
}

type CompactRule struct {
	StartHi    uint64
	StartLo    uint64
	EndHi      uint64
	EndLo      uint64
	JsonOffset uint32
	JsonLen    uint32
}

type AccessLog struct {
	Timestamp int64  `json:"ts"`
	ClientIP  string `json:"src"`
	QueryIP   string `json:"qry"`
	Code      int    `json:"code"`
	Country   string `json:"country,omitempty"`
	Province  string `json:"province,omitempty"`
	City      string `json:"city,omitempty"`
	ISP       string `json:"isp,omitempty"`
	LatencyUs int64  `json:"lat_us"`
}