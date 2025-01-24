add_executable(Dnp3Server ./main.cpp ./json.hpp)
set_target_properties(Dnp3Server PROPERTIES FOLDER cpp/json-scada-drivers)
target_compile_features(Dnp3Server PRIVATE cxx_std_17)
target_include_directories (Dnp3Server PRIVATE 
/json-scada/src/mongo-cxx-driver-lib/include
/json-scada/src/mongo-cxx-driver-lib/include/mongocxx/v_noabi
/json-scada/src/mongo-cxx-driver-lib/include/bsoncxx/v_noabi
)

#add_library(bsoncxx STATIC /json-scada/src/mongo-cxx-driver-lib/lib/bsoncxx-static-rts-x64-v143-md.lib)
#add_library(mongocxx STATIC /json-scada/src/mongo-cxx-driver-lib/lib/mongocxx-static-rts-x64-v143-md.lib)
#set_target_properties(bsoncxx PROPERTIES LINKER_LANGUAGE CXX)
#set_target_properties(mongocxx PROPERTIES LINKER_LANGUAGE CXX)
#target_link_libraries (Dnp3Server PRIVATE 
#opendnp3 
#$<TARGET_OBJECTS:bsoncxx>
#$<TARGET_OBJECTS:mongocxx>
#)


# Use the MSVC variable to determine if this is a Windows build.
set(WINDOWS_BUILD ${MSVC})
if (WINDOWS_BUILD)
target_link_libraries (Dnp3Server 
PRIVATE 
  opendnp3 
PUBLIC
  /json-scada/src/mongo-cxx-driver-lib/lib/bsoncxx-static-rts-x64-v143-md.lib 
  /json-scada/src/mongo-cxx-driver-lib/lib/mongocxx-static-rts-x64-v143-md.lib 
  /json-scada/src/mongo-cxx-driver-lib/lib/mongoc-static-1.0.lib
  /json-scada/src/mongo-cxx-driver-lib/lib/bson-static-1.0.lib
  ws2_32.lib
  Secur32.lib
  Crypt32.lib
  BCrypt.lib
  dnsapi.lib
)
else ()
target_link_libraries (Dnp3Server 
PRIVATE 
  opendnp3 
)
endif ()

install(TARGETS Dnp3Server RUNTIME DESTINATION bin)
