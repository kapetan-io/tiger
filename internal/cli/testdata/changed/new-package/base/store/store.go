package store

type Users struct{}

func (u *Users) Find(id string) string {
	return id
}
