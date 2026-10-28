package main

import (
	"reflect"
	"testing"
)

func TestResolveLabel(t *testing.T) {
	vars := map[string]string{"volume": "37", "system__time": "13:45:08", "nested": "${volume}", "empty": ""}
	for label, want := range map[string]string{"${volume} ${system__time}": "37 13:45:08", "${volume}% / ${volume}%": "37% / 37%", "${missing}": "${missing}", "${nested}": "${volume}", "before ${empty} after": "before  after", "<${volume}>": "<37>"} {
		if got := resolveLabel(label, vars); got != want {
			t.Errorf("%q => %q; want %q", label, got, want)
		}
	}
}
func TestSliderAppearanceStorage(t *testing.T) {
	a := testApp(t)
	interval := .5
	c := a.config()
	expected := Appearance{Effect: "transparent", ImageFit: "stretch", SliderMode: "online", SliderStyle: "filled", SliderInterval: &interval, SliderThumbSize: 24, SliderTrack: "#112233", SliderThumb: "#aabbcc"}
	c.Smart[0].Controls[0].Appearance = expected
	c.Smart[0].Controls[0].Label = "${volume} ${system__time}"
	if err := a.save(c); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadStorage(a.root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Smart[0].Controls[0].Appearance, expected) {
		t.Fatal("appearance not restored")
	}
	if loaded.Smart[0].Controls[0].Label != "${volume} ${system__time}" {
		t.Fatal("template was overwritten")
	}
	bad := 0.0
	c.Smart[0].Controls[0].Appearance.SliderInterval = &bad
	if validate(c) == nil {
		t.Fatal("invalid interval accepted")
	}
}
