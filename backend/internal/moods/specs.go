package moods

type Spec struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Color string `json:"color"`
	Icon  string `json:"icon"`
}

var Order = []string{"happy", "calm", "tired", "sad", "excited"}

var Specs = map[string]Spec{
	"happy":   {Key: "happy", Label: "うれしい", Color: "#F7E7A6", Icon: "Sun"},
	"calm":    {Key: "calm", Label: "おだやか", Color: "#BFE8D4", Icon: "Leaf"},
	"tired":   {Key: "tired", Label: "つかれた", Color: "#D8CEF6", Icon: "Moon"},
	"sad":     {Key: "sad", Label: "しんみり", Color: "#BFDDF3", Icon: "Cloud"},
	"excited": {Key: "excited", Label: "わくわく", Color: "#F7C8D0", Icon: "Sparkles"},
}

func IsValid(key string) bool {
	_, ok := Specs[key]
	return ok
}

func List() []Spec {
	items := make([]Spec, 0, len(Order))
	for _, key := range Order {
		items = append(items, Specs[key])
	}
	return items
}
