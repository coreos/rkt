package netlink

import (
	"testing"
)

func TestClassAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	attrs := QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Handle:    MakeHandle(0xffff, 0),
		Parent:    HANDLE_ROOT,
	}
	qdisc := NewHtb(attrs)
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Htb)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	classattrs := ClassAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    MakeHandle(0xffff, 0),
		Handle:    MakeHandle(0xffff, 2),
	}

	htbclassattrs := HtbClassAttrs{
		Rate:    1234000,
		Cbuffer: 1690,
	}
	class := NewHtbClass(classattrs, htbclassattrs)
	if err := ClassAdd(class); err != nil {
		t.Fatal(err)
	}
	classes, err := ClassList(link, MakeHandle(0xffff, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 1 {
		t.Fatal("Failed to add class")
	}

	htb, ok := classes[0].(*HtbClass)
	if !ok {
		t.Fatal("Class is the wrong type")
	}
	if htb.Rate != class.Rate {
		t.Fatal("Rate doesn't match")
	}
	if htb.Ceil != class.Ceil {
		t.Fatal("Ceil doesn't match")
	}
	if htb.Buffer != class.Buffer {
		t.Fatal("Buffer doesn't match")
	}
	if htb.Cbuffer != class.Cbuffer {
		t.Fatal("Cbuffer doesn't match")
	}
	if err := ClassDel(class); err != nil {
		t.Fatal(err)
	}
	classes, err = ClassList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 0 {
		t.Fatal("Failed to remove class")
	}
	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}
