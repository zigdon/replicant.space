package cmd

func init() {
	mkDeviceCommand(
		"assemble", "Bring the fleet home to the controller's current location without ending the directive", "assemble", nil,
	)
	dirCmd := mkDeviceCommand(
		"directive", "Update the automation policy for a device", "set_directive",
		[]flagDesc{
			{
				name: "new_directive", short: 'n', required: true, jsonKey: "directive",
			},
			{
				name: "configuration", short: 'c', required: false,
				jsonKey: "configuration", mapFlag: true,
			}},
	)
	mkDeviceCommand(
		"launch", "Deploy the fleet and start executing the current directive", "launch", nil,
	)
	mkDeviceCommand(
		"withdraw", "Recall the fleet and pause execution", "withdraw", nil,
	)
}
