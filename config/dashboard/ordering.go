package dashboard

type Ordering string

const (
	OrderingNone  Ordering = ""
	OrderingOrder Ordering = "order"
	OrderingTitle Ordering = "title"
	OrderingKey   Ordering = "key"
)
