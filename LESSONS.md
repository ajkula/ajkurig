# Lessons

What Hordefall and ajkurig taught, so the next game does not fail the same way.

## Performance

- Measure before changing anything, and keep the measurement: a benchmark next to the code, before and after numbers in the commit or the README. Never blame the machine before profiling (`go test -bench X -cpuprofile cpu.out`, then `go tool pprof -top`).
- Draw thousands of shapes through `draw.SpriteBatch` (one `DrawTriangles32` per batch). `vector` paths per shape froze Hordefall (2.9 FPS in an effects storm, 60 after the switch).
- Spread heavy per-entity work over frames: separation every second frame, ground contact every twelfth, staggered by index (`spatial.IsTurn`). 10 000 discs went from 28 ms to 3.3 ms per tick.
- Query neighbours through `spatial.Grid` with a cap (`AppendNearbyLimited`), compare squared distances, take the square root only for real overlaps.
- Keep hot data in flat arrays (one slice per field) and remove by swapping with the last element.
- Track what changed instead of scanning everything: the ground refresh keeps a list of changed cells and of glowing cells (1 ms to 53 us per frame).
- Skip inert work early: ground cells that neither emit nor expire are not visited.
- `time.Now()` costs about 300 ns on Windows: time one frame in eight, not every call.
- Reuse buffers and maps every frame (shader uniforms, candidate lists); no allocation in hot loops.

## Correctness

- Every optimisation ships with a test that runs the old algorithm next to the new one and compares the results exactly.
- Watch `iota` in mixed const blocks: a line without a value repeats the previous expression, it does not restart. Give counts explicitly (`kindCount = LastKind + 1`) and keep each enum in its own block.
- Reactions must consume what triggered them (an inferno removes the oil), or chains never end.
- Never crash on missing data: a missing key falls back to English then to the key, a broken language file is skipped, a broken settings or controls file falls back to defaults, a missing glyph falls back to the next font.
- Save remapped controls by action name, not by index, so adding an action never scrambles old saves.
- The demo must never record a score: it runs the real simulation, so guard every reward with `frame.IsDemo`.

## Testing

- The whole game loop runs headless: `app.Step(controls)` drives screens without a window, so menus, runs, scores and saves are tested end to end.
- Test behaviour, not code shape: "fire spreads then grass grows back", "a finished run opens the name entry", never "this variable equals this literal".
- Long simulated runs (minutes at 60 ticks) with every mechanic on catch instability that unit tests miss.

## Audio

- One persistent pool of voices, notes only restart envelopes; no allocation in the audio callback.
- Contain levels at the source (voice volumes, layer gains), then a soft clip at the end; test peak and RMS of rendered audio, never by ear first.
- The game thread never waits for the audio thread: it only drops settings, sounds and signals into a short lock, and the audio callback copies them at the start of each buffer and renders without holding the lock. Holding the lock while rendering made every tick wait up to a whole buffer and froze the game during song crossfades.
- Adaptive layers: every channel always plays in time, only its gain follows the signal, so layers enter on the beat.
- Play sounds only through ids with a minimum interval, so a horde dying at once does not stack hundreds of pops.

## Interface

- Every text is measured in every language by a test; reword when it overflows instead of shrinking fonts.
- Hints name the real buttons of the current device (`{confirm}`, `{back}`), never a hard-coded key.
- A short lock after each screen change (0.45 s) stops one press from triggering two menus.

## Workflow

- When the same problem shows up twice, stop patching: profile or reread, write the plan, then change strategy.
- Keep framework code agnostic: a game fills tables and mounts modules; a mechanic proven in a game moves into the framework as a module.
