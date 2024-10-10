package main

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type IdentGen struct {
	pos int
}

func (g *IdentGen) Next() string {
	ident := ""
	pos := g.pos

	for {
		ident = string(chars[pos%len(chars)]) + ident
		pos = pos / len(chars)
		if pos == 0 {
			break
		}
		pos -= 1
	}

	g.pos++

	return ident
}
