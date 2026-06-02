#include <iostream>
#include <vector>
#include <sstream>
#include <algorithm>
#include <string>

using namespace std;

int main() {
    string input;
    if (getline(cin, input)) {
        vector<int> arr;
        stringstream ss(input);
        string item;
        while (getline(ss, item, ',')) {
            arr.push_back(stoi(item));
        }
        sort(arr.begin(), arr.end());
        for (size_t i = 0; i < arr.size(); ++i) {
            cout << arr[i];
            if (i < arr.size() - 1) cout << ",";
        }
    }
    return 0;
}
