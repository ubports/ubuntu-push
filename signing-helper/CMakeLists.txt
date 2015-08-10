cmake_minimum_required(VERSION 2.8)
SET (EXAMPLES_TARGET ubuntuoneauth-examples)

SET (SIGNING_EXE "signing-helper")

find_package (PkgConfig REQUIRED)
pkg_check_modules(UBUNTUONE REQUIRED ubuntuoneauth-2.0)
add_definitions(${UBUNTUONE_CFLAGS} ${UBUNTUONE_CFLAGS_OTHER})


# Qt5 bits
SET (CMAKE_INCLUDE_CURRENT_DIR ON)
SET (CMAKE_AUTOMOC ON)
find_package(Qt5Core REQUIRED)

SET (SIGNING_SOURCES signing-helper.cpp)
SET (SIGNING_HEADERS signing.h)

add_executable (${SIGNING_EXE}
  ${SIGNING_SOURCES}
  ${SIGNING_HEADERS})
qt5_use_modules (${SIGNING_EXE} DBus Network)

target_link_libraries (${SIGNING_EXE}
  ${UBUNTUONE_LDFLAGS})



add_custom_target(examples-valgrind
    COMMAND "valgrind --tool=memcheck ${CMAKE_CURRENT_BINARY_DIR}/${SIGNING_EXE}"
    DEPENDS ${SIGNING_EXE}
)

add_custom_target(examples-valgrind-leaks
    COMMAND "valgrind --tool=memcheck --track-origins=yes --num-callers=40 --leak-resolution=high --leak-check=full ${CMAKE_CURRENT_BINARY_DIR}/${SIGNING_EXE}"
    DEPENDS ${SIGNING_EXE}
)

INSTALL_TARGETS( "lib/ubuntu-push-client/" ${SIGNING_EXE})
