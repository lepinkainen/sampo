# Classification golden-test fixtures

Images used by `TestClassifyGolden` to lock expected CLIP tags. All are in the
**public domain** so they can be redistributed in this repo without conditions.
Each was fetched from Wikimedia Commons via `Special:FilePath` (512px).

| File | Subject | Source (Wikimedia Commons) | License | Author |
|------|---------|----------------------------|---------|--------|
| `person_lincoln.jpg` | person | `Abraham_Lincoln_O-77_matte_collodion_print.jpg` | Public domain | Alexander Gardner |
| `person_einstein.jpg` | person | `Albert_Einstein_Head.jpg` | Public domain | Orren Jack Turner |
| `animal_eagle.jpg` | animal | `Bald Eagle (30319996332).jpg` | Public domain (USFWS) | U.S. Fish & Wildlife Service |
| `vehicle_car.jpg` | vehicle | `1910Ford-T.jpg` | Public domain | Harry Shipler |

To view a source page: `https://commons.wikimedia.org/wiki/File:<filename>`

## Adding fixtures

Prefer **Public Domain / CC0** images so no attribution machinery is needed.
Add the file here with its license + author, then add a case to `goldenCases`
in `../classifier_golden_test.go` with the tag it must (and must not) produce.
