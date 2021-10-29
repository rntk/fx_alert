package patterns

type Name string

const (
	PinBar Name = "pinbar"
)

type Sentiment string

const (
	Bull    Sentiment = "bull"
	Bear    Sentiment = "bear"
	Neutral Sentiment = "neutral"
)

type Pattern struct {
	Name      Name
	Sentiment Sentiment
}
