#include "cap/codec.h"

#include <google/protobuf/io/coded_stream.h>
#include <google/protobuf/io/zero_copy_stream_impl_lite.h>

namespace cap {

std::vector<uint8_t> MarshalDeterministic(const google::protobuf::Message& msg) {
  const size_t size = msg.ByteSizeLong();
  std::vector<uint8_t> buf(size);
  if (size == 0) return buf;

  google::protobuf::io::ArrayOutputStream array_stream(buf.data(),
                                                        static_cast<int>(size));
  google::protobuf::io::CodedOutputStream coded_stream(&array_stream);
  coded_stream.SetSerializationDeterministic(true);
  if (!msg.SerializeToCodedStream(&coded_stream)) {
    return {};
  }
  return buf;
}

std::vector<uint8_t> MarshalUnsignedForSignature(
    const cordum::agent::v1::BusPacket& packet) {
  cordum::agent::v1::BusPacket clone;
  clone.CopyFrom(packet);
  clone.clear_signature();
  return MarshalDeterministic(clone);
}

}  // namespace cap
