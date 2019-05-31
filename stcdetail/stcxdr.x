
// Local XDR data structures defined for use in STC library.

struct XdrTxResult {
  Hash txhash;
  TransactionEnvelope env;
  TransactionResult result;
  TransactionMeta resultMeta;
};
