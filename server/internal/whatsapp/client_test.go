package whatsapp

import (
	"crypto/sha256"
	"reflect"
	"testing"
)

func TestMatchOptionsTurnsVoteHashesBackIntoOptionNames(t *testing.T) {
	options := []string{"✅ Confirmo", "❌ Não posso"}
	hash := sha256.Sum256([]byte("❌ Não posso"))
	unknown := sha256.Sum256([]byte("outra coisa"))

	got := matchOptions(options, [][]byte{hash[:], unknown[:]})
	if want := []string{"❌ Não posso"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("matchOptions = %v, want %v", got, want)
	}
}
