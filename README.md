# [WIP] oggopus

Native reader for .ogg files containing opus encoded data.

oggreader based on https://github.com/jfreymuth/oggvorbis

## Description

Example:
```go
        file, _ := os.Open("sample.ogg")
        reader, _ := opus_reader.NewOpusReader(file)
        for {
            packet, err := reader.NextPacket()

            // do packet data processing

            if err != nil {
                break
            }
        }       
```

## TODO
- improve, refactor
