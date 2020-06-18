# [WIP] oggopus

Native decoder for ogg/opus.

oggreader based on https://github.com/jfreymuth/oggvorbis

## Description

You can use this lib to read the .ogg files containing an opus encoded data.

Example:
```go
        file, _ := os.Open("sample.ogg")
        reader, _ := opus_reader.NewOpusReader(file)
        for {
            packet, err := reader.NextPacket()
            // do packet data processing
            if reader.LastPacket {
                break
            }
        }       
```

## TODO
- better docs
- more tests
- refactor
- improvements
