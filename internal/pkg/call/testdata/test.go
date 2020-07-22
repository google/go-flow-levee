package test

func NoArgs() {}

func OneArg(one int) {}

func CallNoArgs() {
	NoArgs()
}

func CallOneArg() {
	OneArg(0)
}
