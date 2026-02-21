cmake_minimum_required(VERSION 3.20)
project(cap_cpp_sdk LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(NOT DEFINED PROTO_ROOT)
  set(PROTO_ROOT "${CMAKE_SOURCE_DIR}/../../proto")
endif()

# Prefer pre-generated C++ stubs under /cpp to avoid protoc at build time.
set(CPP_STUB_DIR "${CMAKE_SOURCE_DIR}/../../cpp")

find_package(Protobuf REQUIRED)
find_package(OpenSSL REQUIRED)

file(GLOB CAP_PROTO_SRCS
  "${CPP_STUB_DIR}/cordum/agent/v1/*.pb.cc"
)
file(GLOB CAP_PROTO_HDRS
  "${CPP_STUB_DIR}/cordum/agent/v1/*.pb.h"
)

add_library(cap_cpp
  ${CAP_PROTO_SRCS}
  ${CAP_PROTO_HDRS}
  src/client.cpp
  src/worker.cpp
  src/signing.cpp
  src/codec.cpp
  src/middleware.cpp
  src/heartbeat.cpp
  src/progress_cancel.cpp
  src/agent.cpp
)

target_include_directories(cap_cpp
  PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CPP_STUB_DIR}
    ${PROTOBUF_INCLUDE_DIRS}
)

target_link_libraries(cap_cpp
  PUBLIC ${Protobuf_LIBRARIES}
  PUBLIC OpenSSL::Crypto
)

option(CAP_BUILD_EXAMPLE "Build example programs" OFF)

if(CAP_BUILD_EXAMPLE)
  add_executable(cap_example_echo examples/echo_main.cpp)
  target_link_libraries(cap_example_echo cap_cpp)
endif()

option(CAP_BUILD_TESTS "Build unit tests" OFF)

if(CAP_BUILD_TESTS)
  enable_testing()
  find_package(GTest REQUIRED)

  add_executable(cap_signing_test test/signing_test.cpp)
  target_link_libraries(cap_signing_test cap_cpp GTest::gtest GTest::gtest_main)
  add_test(NAME signing_test COMMAND cap_signing_test)

  add_executable(cap_codec_test test/codec_test.cpp)
  target_link_libraries(cap_codec_test cap_cpp GTest::gtest GTest::gtest_main)
  add_test(NAME codec_test COMMAND cap_codec_test)

  add_executable(cap_middleware_test test/middleware_test.cpp)
  target_link_libraries(cap_middleware_test cap_cpp GTest::gtest GTest::gtest_main)
  add_test(NAME middleware_test COMMAND cap_middleware_test)

  add_executable(cap_heartbeat_test test/heartbeat_test.cpp)
  target_link_libraries(cap_heartbeat_test cap_cpp GTest::gtest GTest::gtest_main)
  add_test(NAME heartbeat_test COMMAND cap_heartbeat_test)

  add_executable(cap_progress_cancel_test test/progress_cancel_test.cpp)
  target_link_libraries(cap_progress_cancel_test cap_cpp GTest::gtest GTest::gtest_main)
  add_test(NAME progress_cancel_test COMMAND cap_progress_cancel_test)

  add_executable(cap_agent_test test/agent_test.cpp)
  target_link_libraries(cap_agent_test cap_cpp GTest::gtest GTest::gtest_main)
  add_test(NAME agent_test COMMAND cap_agent_test)
endif()
