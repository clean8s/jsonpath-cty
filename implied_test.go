package peek

import (
	"testing"
	"fmt"
)

type A struct {
	Name string
	Age int
	Something map[string]bool
}

func TestImpliedStruct(t *testing.T) {
	type AA struct {
		X int
		Y int
	}

	type Person struct {
		Name string
		Age int
		Coords *AA
	}

	john := New(Person{"John", 25, nil})
	fmt.Println(john, "\n", john.Type())

	tup := Tuple(Num(1), Num(2), True)
	fmt.Println("tup =", tup)
	fmt.Println("tup.Type() =", tup.Type())

	fmt.Println("john == tup:", john.Equals(tup))

	custom := New(AA{X: 3, Y: 6})
	fmt.Println(custom, custom.Type())
	O := NewObjBuilder().Put("A", True).Put("B", Num(3)).Value()

	T := Tuple(Num(3), O)
	fmt.Println(T)
	fmt.Println(T.Search("$..A"))
	fmt.Println(T.Children())
	for _, item := range T.Children() {
		fmt.Println(item)
	}
	//Type := cty.TupleVal([]cty.Value{cty.NumberIntVal(3), cty.NumberIntVal(5)})
	//VV, _ := gocty.ToCtyValue(A{4, 5}, Type.Type())
	//fmt.Println(Value(VV))

}
