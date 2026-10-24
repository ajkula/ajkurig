# Body roadmap

Goal: build a caricatured wrestler fast. The artist draws the head (the likeness); the body comes from a few numbers and turns correctly at any angle, lying, lifted or upside down.

Each step is test-driven: write the failing tests listed, make them pass, keep every existing test green, then show the result before the next step.

## 1. Limb volumes

- [x] A part can carry a `body` instead of a drawing: a tapered tube along its bone with rounded ends, a start and end width and a depth ratio for an oval section. It renders through the solid path.
  - Tests: a body part seen from the front is as wide as its width; turned 90 degrees it is as wide as its depth; seen end-on it is an oval; the body is written and read back; a part without a body renders as before.

## 2. Clothes

- [ ] Colour bands along the bone (sleeve then skin, trouser then shoe) and sectors around it (a tie in front, a stripe on the side).
  - Tests: a band colours only its stretch of the bone; a front sector is seen from the front and hidden from the back.

## 3. Shading

- [ ] Each body voxel keeps its surface normal; a fixed light colours it with the lit, base or shadow swatch of its material, whatever the pose.
  - Tests: the side toward the light uses the lit swatch, the other side the shadow swatch; turning the part moves the shadow with it.

## 4. Body shapes

- [ ] Torso and pelvis shapes: shoulder and hip widths, belly and chest bulges; hands and feet as end blobs (fist, glove, shoe).
  - Tests: a belly bulge widens the torso only in front; a fist is wider than its forearm.

## 5. Body files

- [ ] A `.body` text file describes proportions (height, leg length, head scale) and every part's shape and clothes; `spritetool build hero.spr suit.body micron.body` creates or updates a humanoid, the later files overriding the earlier ones, and keeps the drawn head and its views.
  - Tests: building twice gives the same sprite; a drawn head survives a rebuild; the common moves compose on a built body with the feet planted.

## 6. First caricature

- [ ] One wrestler built from a body file and a drawn head, with the common wrestling moves, shown in the arena at near and far depth.
