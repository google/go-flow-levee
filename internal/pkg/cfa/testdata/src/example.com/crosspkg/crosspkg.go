package crosspkg

import "example.com/cfa"

func TestCrossPkg(e interface{}) { // want TestCrossPkg:"genericFunc{ sinks: <0>, taints: <<>> }"
	cfa.OneParamSinkWrapper(e)
}
