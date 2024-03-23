/*
Package bits - bit and bytes reading and writing including Golomb codes and EBSP.
All readers and writers accumulate errors in the sense that they will stop reading or writing at the first error.
The first error, if any, can be retrieved with AccError().

Beyond plain bit reading and writing, reading and writing of ebsp (Encapsulated Byte Sequence Packets) is supported.
EBSP uses insertion of start-code emulation prevention bytes 0x03 and is used in MPEG video standards from AVC (H.264) and forward.
*/
package bits
