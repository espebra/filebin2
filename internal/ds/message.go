package ds

type Message struct {
	Id    int    `json:"id"`
	Level string `json:"level"`
	Text  string `json:"text"`
}
