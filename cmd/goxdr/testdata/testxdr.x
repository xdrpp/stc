typedef int int32;

union U1 switch (int32 type) {
default:
      void;
};

enum E1 {
     E1_zero = 0,
     E1_one = 1
};

union U2 switch (int32 type) {
case E1_zero:
     void;
};

typedef bool Bool;
typedef Bool myBool;

union U3 switch (Bool b) {
case TRUE:
     int val;
default:
     void;
};

union U4 switch (myBool b) {
case TRUE:
     int val;
default:
     void;
};

typedef U2 A1[5];

struct A2 {
     A1 a[4];
     E1 e[4];
     myBool b[4];
     U2 u[4];
};
