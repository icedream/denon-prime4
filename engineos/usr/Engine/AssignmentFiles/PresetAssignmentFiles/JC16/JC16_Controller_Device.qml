import airAssignments 1.0
import InputAssignment 0.1
import OutputAssignment 0.1
import Device 0.1
import QtQuick 2.0

Device {
	id: device

	property real gamma: 3.5
	property real padGamma: 3.5

	controls: []
	useGlobalShift: false
	numberOfLayers: 0

	property string deviceInfo: ""
	property string currentMixerFirmwareVersion

	///////////////////////////////////////////////////////////////////////////
	// Setup

	function queryAbsoluteControls() {
		Midi.sendSysEx("F0 00 02 0B 7F 0B 04 00 00 F7")
	}

	function sendInitializationMessage() {
		Midi.sendSysEx("F0 00 02 0B 7f 0B 60 00 04 04 01 01 03 F7")
	}

	property Timer initPhaseEndTimer: Timer {
		interval: 1000
		repeat: false
		onTriggered: {
			device.isInitializing = false
			queryAbsoluteControls()
		}
	}

	property bool isInitializing: false

	Component.onCompleted: {
		Midi.sendSysEx("F0 7E 00 06 01 F7")

		isInitializing = true

		requestPowerOnButtonState()

		sendInitializationMessage();

		initPhaseEndTimer.start()
	}

	Component.onDestruction: {
		Midi.sendNoteOff(1, 117)
	}

	// Dec to Hex Conversion
	function d2h(d){
		return (+d).toString(16).toUpperCase()
	}

	function midiColorChannel(c, gamma){
		return d2h(Math.min(127, Math.max(0,Math.floor(Math.pow(c, gamma) * 127))))
	}

	function mapColor(color) {
		return Qt.rgba(color.r, color.g, color.b , color.a)
	}

	function midiColor(color, gamma) {
		var c = mapColor(color)
		return midiColorChannel(c.r, gamma)+ " " +  midiColorChannel(c.g, gamma) + " " +  midiColorChannel(c.b, gamma)
	}

	function sendNoteOn(channel, index, value) {
		Midi.sendNoteOn(channel, index, value)
	}

	function sendSimpleColor(channel, index, value) {
		if (value === 0) {
			Midi.sendNoteOff(channel, index)
		}
		else {
			Midi.sendNoteOn(channel, index, value)
		}
	}

	//Color Send Function
	function sendColor(channel, index, color)
	{
		var g = device.gamma
		if(index >= 15 && index <= 23) {
			g = device.padGamma
		}

		var sysEx = "F0 00 02 0B 7F 0B 03 00 05 " + d2h(channel) + " " + d2h(index) + " " + midiColor(color, g)+" F7"
		Midi.sendSysEx(sysEx)

	}

	function requestPowerOnButtonState() {
		Midi.sendSysEx("F0 00 02 0B 7F 0B 42 00 00 F7")
	}

	function sysExToIntList(sysExString)
	{
		var valueList = sysExString.split(" ")
		var result = []

		for(var i = 0; i < valueList.length; ++i) {
			result.push(parseInt(valueList[i], 16))
		}

		return result
	}

	function sysEx(sysExString) {
		console.info("Received SysEx:", sysExString)
		var valueList = sysExToIntList(sysExString)
		var result = ""
		if(valueList[1] === 0x00 && valueList[2] === 0x02 && valueList[3] === 0x0B && valueList[4] === 0x00 && valueList[6] === 0x42)
		{
			if(valueList[9] === 0x0) {
				console.log("No special power on request")
			}

			else if(valueList[9] === 0x1) {
				console.log("Request test-mode entry")
				quitToTestApp()
			}
		}
		else if(valueList[1] === 0x7E && valueList[2] === 0x00 && valueList[3] === 0x06 && valueList[4] === 0x02)
		{
			var i
			for(i = 0; i < 4; ++i) {
				result += valueList[i + 11]
				if(i === 1) {
					result += "."
				}
			}
			deviceInfo = result
		}
	}
}

