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

file(GLOB CAP_PROTO_SRCS
  "${CPP_STUB_DIR}/coretex/agent/v1/*.pb.cc"
)
file(GLOB CAP_PROTO_HDRS
  "${CPP_STUB_DIR}/coretex/agent/v1/*.pb.h"
)

add_library(cap_cpp
  ${CAP_PROTO_SRCS}
  ${CAP_PROTO_HDRS}
  src/client.cpp
  src/worker.cpp
)

target_include_directories(cap_cpp
  PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CPP_STUB_DIR}
    ${PROTOBUF_INCLUDE_DIRS}
)

target_link_libraries(cap_cpp
  PUBLIC ${Protobuf_LIBRARIES}
)

option(CAP_BUILD_EXAMPLE "Build example programs" OFF)

if(CAP_BUILD_EXAMPLE)
  add_executable(cap_example_echo examples/echo_main.cpp)
  target_link_libraries(cap_example_echo cap_cpp)
endif()
