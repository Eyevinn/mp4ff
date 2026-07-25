/*
Package av1 decodes (parses) and encodes (writes) the AV1 CodecConfigurationRecord
and parses the OBU (Open Bitstream Unit) structure of AV1 streams.

Metadata OBUs are handled in layers: MetadataOBU is any metadata OBU (a metadata_type
with its payload and trailing_bits), ITUTT35 is the metadata_itu_t_t35 body carried by
one, and CreateCTA608MetadataOBU/ExtractCTA608 form the CTA-608 closed-caption layer on
top of those. The CTA-608 payload is the same cc_data() as in an AVC/HEVC SEI message,
so it is built from the same input as sei.CreateCTA608SEIMessage.
*/
package av1
