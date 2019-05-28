
// Local XDR data structures defined for use in STC library.

struct XdrTxResult {
  TransactionEnvelope env;
  TransactionResult result;
  TransactionMeta resultMeta;
  Hash txhash;
};
