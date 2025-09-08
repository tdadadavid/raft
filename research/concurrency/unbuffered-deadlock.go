package main

func Run() {
	c := make(chan bool)
	c <- true
	<- c
}