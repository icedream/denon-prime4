# Initialization Handshake Hypothesis - MIDI SysEx Trigger

## Theory

The FunctionFS USB endpoints on the Prime 4 in Computer Mode may require an **initialization message via MIDI** before the device-side application (`planck-remote-screen`) activates and processes USB MessageBlock/SMEX protocol messages.

## Evidence

### 1. planck-remote-screen Has MIDI Input Handling
- Binary strings: `MIDIINPUT`, `N4juce::MidiInputCallback`, `N4juce::MidiMessageCollector`
- JUCE framework is compiled in with full MIDI input support
- The application actively listens for MIDI messages (not passive)

### 2. Engine DJ Sends Denon SysEx Commands on Startup
- Found QML code in Engine DJ.exe that sends: `F0 00 02 0B 00 0C 05 00 01 0%1 F7`
- This is a **Denon manufacturer SysEx** (manufacturer ID `00 02 0B`)
- Sent during initialization for bank reset on jog wheels (command byte `0x0C`)
- Suggests Engine DJ sends SysEx commands to initialize the Prime 4 state during startup

### 3. Serato SDK Also Uses MessageBlock + MIDI
- The `serato_device_akaisdk.dll` contains all 14 SMEX message type strings
- Bidirectional communication expected (device initiates certain messages too)
- Standard practice: Host sends MIDI command → Device resets/unblocks FunctionFS → Then MessageBlock communication begins

### 4. Current Blockage Pattern
- FunctionFS endpoints accept data writes into DMA buffers
- But completion callbacks never fire (`ffs_epfile_io` blocks indefinitely)
- This suggests the endpoints need to be "armed" or "unlocked" by receiving some kind of command
- MIDI SysEx is the only other USB interface available (besides FunctionFS itself)

## Hypothesis

**When `planck-remote-screen` starts, it requires an initialization SysEx message from the host (via MIDI) to:**
1. Establish connection state
2. Enable or unlock the FunctionFS endpoints
3. Start listening for SMEX MessageBlock protocol messages

The initialization sequence on first connection would be:

```
HOST                              PRIME 4
  |                                  |
  |-- [MIDI SysEx Init cmd] -------->|  planck-remote-screen wakes up
  |                                  |  FunctionFS endpoints become active
  |<-- [SMEX protocolversion] -------|  Device sends version via EP3 IN
  |
  |-- [SMEX protocolversion] ------->|  Host confirms version via EP4 OUT
  |
  |<-- [SMEX version] ----------------|  Device sends firmware version
  |
  | ... rest of SMEX protocol ...     |
```

## Testable Predictions

1. Sending ANY Denon SysEx command to the Prime 4 before attempting SMEX MessageBlock communication might unblock the endpoints
2. The command might be a simple "online" or "acknowledge" type message (e.g., device ID inquiry response)
3. Once the MIDI SysEx is received and processed, subsequent USB transfers to EP4 OUT and EP3 IN should complete

## Next Steps

1. **Identify the exact initialization SysEx command** from the captured MIDI traffic of Engine DJ connecting to Prime 4
2. **Test hypothesis** by sending the SysEx command and then attempting SMEX handshake
3. **Analyze the MIDI message handler** in planck-remote-screen via Ghidra to find what command unlocks FunctionFS
4. **Monitor planck-remote-screen logs** (if available) when Engine DJ connects to see if MIDI messages are logged

## Related Files

- Jog wheel SysEx documentation: `/docs/reveng/computer-mode/03-jog-wheel-sysex.md`
- SMEX protocol: `/docs/reveng/computer-mode/04-main-screen-smex.md`
- FunctionFS kernel issue (alternative theory): `/docs/reveng/computer-mode/07-functionfs-kernel-issue.md`
