package wclogs

// classIDCanHeal defines a collection of classes capable of healing
var classIDCanHeal = []int{2, 5, 6, 7, 9}

// Character represents character info
type Character struct {
	ID      int
	Name    string
	Server  string
	Region  string
	ClassID int
}

// Slug returns printable Character identifier
func (t *Character) Slug() string {
	return t.Name + " " + t.Region + "-" + t.Server
}

// CanHeal returns true if Character should also be tracked as a healer
func (t *Character) CanHeal() bool {
	for _, classID := range classIDCanHeal {
		if classID == t.ClassID {
			return true
		}
	}

	return false
}
