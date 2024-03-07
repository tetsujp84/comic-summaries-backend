package entity

// Comic は漫画のエンティティを表します。
type Comic struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Synopsis   string `json:"synopsis"`
	Attraction string `json:"attraction"`
	Conclusion string `json:"conclusion"`
	Genre      string `json:"genre"`
	Characters string `json:"characters"`
}
