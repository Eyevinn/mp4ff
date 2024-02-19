/*
Package bits - bit and bytes reading and writing including Golomb codes and EBSP.
All readaer and writer accumulates errors in the sense that they will stop reading or writing at the first error.
The first error can be retrieved with AccError().

Beyond plain bit reading and writing, reading and writing of ebsp (Encapsulated Byte Sequence Packets)
with start-code emulation prevention bytes and exponential Golomb codes as used in the AVC/H.264 and HEVC video coding standards.
*/
package bits
