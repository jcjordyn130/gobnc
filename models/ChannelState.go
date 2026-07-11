package models

type ChannelState struct {
	Name  string
	Topic string
	Modes string

	// This is *technically* a UNIX timestamp but it's getting formatted
	// into a string anyways, so why bother?
	CreationTime string
}
