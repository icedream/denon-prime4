# StateMap Property Paths

These are Denon's internal `StateMap` key paths (slash-delimited strings)
observed in the firmware QML files for Computer Mode display assignments.

The host reads these from its own internal state and uses them to drive
the display rendering pipeline.

## Deck-specific paths

Replace `%d` with the deck number (1–4) or `%s` with Left/Right.

### Track metadata

```
/Engine/Deck%d/Track/SongLoaded           bool   - track is loaded
/Engine/Deck%d/Track/TrackName            string - track title
/Engine/Deck%d/Track/ArtistName           string - artist name
/Engine/Deck%d/Track/AlbumName            string - album name
/Engine/Deck%d/AlbumArt                   bytes  - album artwork (JPEG/PNG blob)
/Engine/Deck%d/Track/SampleRate           double - sample rate (Hz)
/Engine/Deck%d/Track/TrackData/TrackLength       double - duration (seconds)
/Engine/Deck%d/Track/TrackData/PlayheadPosition  double - current position (seconds)
```

### Playback state

```
/Engine/Deck%d/Play                       bool   - playing
/Engine/Deck%d/PlayState                  double - 0=stopped, 1=playing, 2=paused
/Engine/Deck%d/PlayStatePath              string
/Engine/Deck%d/CurrentBPM                 double - current BPM
/Engine/Deck%d/Speed                      double - pitch-adjusted speed
/Engine/Deck%d/SpeedNeutral               double
/Engine/Deck%d/SpeedOffsetDown            double
/Engine/Deck%d/SpeedOffsetUp              double
/Engine/Deck%d/SpeedRange                 double
/Engine/Deck%d/SpeedState                 double
/Engine/Deck%d/SyncMode                   double
/Engine/Deck%d/DeckIsMaster               bool
/Engine/Deck%d/Track/KeyLock              bool
```

### Loop state

```
/Engine/Deck%d/Track/AutoLoopIndex        int    - current loop size index
/Engine/Deck%d/Track/AutoLoopLabel%d      string - loop size label (e.g. "1/4")
/Engine/Deck%d/Track/LoopEnableState      bool   - loop active
/Engine/Deck%d/Track/Loop/Active          bool
/Engine/Deck%d/Track/Loop/LoopEnabledPosition  double
/Engine/Deck%d/Track/Loop/LoopOutPosition      double
/Engine/Deck%d/Track/Loop/QuickLoop1..8        double
```

### Beat jump

```
/Engine/Deck%d/Track/BeatJump/BeatJumpIndex    int    - current beat jump size index
/Engine/Deck%d/Track/BeatJump/BeatJumpLabel%d  string - size label
```

### Slip mode

```
/Engine/Deck%d/Track/SlipModeActive       bool
```

### Visual / jog

```
/Engine/Deck%d/JogColor                   color  - deck accent color
/Engine/Deck%d/ExternalMixerVolume        double
/Engine/Deck%d/ExternalScratchWheelTouch  bool
/Engine/Deck%d/DeckIsMaster               bool
```

### Realtime position (high frequency)

```
/Private/Deck%d/MidiSamplePosition        int    - sample index (updated ~83 Hz)
```

## Mixer paths

```
/Engine/Mixer/Channel%d/PFL               bool   - pre-fader listen
/Engine/Mixer/Channel%d/Line              bool   - line-in mode active
/Engine/Mixer/Channel%d/AutoGain          double
/Engine/Mixer/AutoPFLDeckIndex            int
/Engine/Mixer/Deck%d/...                  (various)
```

## Client / UI paths

```
/Client/Preferences/CueSolo               bool
/Client/Preferences/ScreenBrightnessPluggedIn  string - "Low"/"Mid"/"High"/"Max"
/Client/Librarian/DevicesController/CurrentDeviceArtwork  bytes
/Client/Assignments/Display/DeviceInfo    string - firmware version (for capability checks)
/GUI/Decks/Deck%s/ActiveDeck              string - active deck number for layer
/GUI/Scripted/RunningDark                 bool   - dark/sleep mode
/Configuration/ComputerMode               bool   - computer mode active (set by host)
```

## Notes

- These paths are used by the host to track state
  internally and drive the display rendering QML
- In Computer Mode the host **emits** many of these values back to the device
  if the device requests them (via SMEX or StateMap subscription)
- The `/Private/...` paths are high-frequency realtime values not suitable for
  network transmission - they are computed locally and sent as MIDI pitch messages
  instead of StateMap updates
- `/Configuration/ComputerMode` is a key StateMap flag - setting it `true`
  activates the computer mode UI on the device
