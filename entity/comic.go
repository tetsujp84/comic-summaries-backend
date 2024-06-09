package entity

// Comic は漫画のエンティティを表します。
type Comic struct {
	ID         string `json:"id" dynamodbav:"ID"`
	Title      string `json:"title" dynamodbav:"Title"`
	Synopsis   string `json:"synopsis" dynamodbav:"Synopsis"`
	Attraction string `json:"attraction" dynamodbav:"Attraction"`
	Spoilers   string `json:"spoilers" dynamodbav:"Spoilers"`
	Genre      string `json:"genre" dynamodbav:"Genre"`
	Characters string `json:"characters" dynamodbav:"Characters"`
	ImagePath  string `json:"image_path" dynamodbav:"ImagePath"`
}
