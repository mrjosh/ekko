package hub

type Packet struct {
	Type   string      `json:"type,omitempty"`
	Data   interface{} `json:"data,omitempty"`
	UserId string      `json:"user_id,omitempty"`
	RoomId string      `json:"room_id,omitempty"`
}
