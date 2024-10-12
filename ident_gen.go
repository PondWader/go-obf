package main

const CHARSET_LOWERCASE = "abcdefghijklmnopqrstuvwxyz"
const CHARSET_UPPERCASE = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const CHARSET_ALPHABET = CHARSET_LOWERCASE + CHARSET_UPPERCASE

type IdentGen struct {
	charSet string
	pos     int
}

func NewIdentGen(charSet string) IdentGen {
	return IdentGen{charSet, 0}
}

func (g *IdentGen) Next() string {
	ident := ""
	pos := g.pos
	chars := g.charSet

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
