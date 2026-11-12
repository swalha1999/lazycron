package ui

import "testing"

func TestBuildJobRejectsUnsafeLabels(t *testing.T) {
	cases := map[int]string{fieldName: "Disk 90% alert", fieldProject: "50%"}
	for field, value := range cases {
		f := newForm(nil)
		f.inputs[fieldName].SetValue("Backup")
		f.inputs[fieldCommand].SetValue("true")
		f.inputs[field].SetValue(value)
		if _, err := f.buildJob(); err == nil {
			t.Errorf("field %d = %q: expected an error", field, value)
		}
	}
}
