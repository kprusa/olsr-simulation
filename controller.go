package main

// Controller is aware of the entire network typology and acts as a wireless network.
// Only used for the simulation (a real ad-hoc network would not have a centralized controller).
type Controller struct {
	topology NetworkTypology
}

func (c *Controller) Initialize() {

}

func (c *Controller) Start() {

}
