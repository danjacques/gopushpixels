## Notes on PixelPusher protocol.

Most of this protocol is reverse-engineered from code in the 
[PixelPusher-java](https://github.com/robot-head/PixelPusher-java) repository.

PixelPusher uses UDP packets. A packet is either;:
* A command.
* A LED state.

Each packet begins with a 4-byte big-endien number identifying the packet's
index. This index is unique per packet and increments with each successive
packet.

Command packets are formatted:
* Packet index (4 bytes).
* Command magic sequence (16 bytes).
* Command identification (1 byte).
* Command data (remainder).

LED states are formatted:
* Packet index (4 bytes).
* Repeated sequences of:
    * Strip Number (1 byte)
    * Strip State, consisting of PixelsPerStrip sequences of either:
        * RGBOW (if SFLAG_RGBOW is set)
            * Red (1 byte)
            * Green (1 byte)
            * Blue (1 byte)
            * Orange (3 bytes, all identical)
            * White (3 bytes, all identical)
        * RGB
            * Red (1 byte)
            * Green (1 byte)
            * Blue (1 byte)
