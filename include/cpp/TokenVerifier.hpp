#pragma once
#include <string>
#include <string_view>
#include <sw/redis++/redis++.h>
using namespace std;
// Forward declaration
string verifyUploadImageGetUserId(string_view token, const string& pubKey,sw::redis::Redis& redis);