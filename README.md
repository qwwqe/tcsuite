# tcsuite
Suite for gathering, processing and serving textual content

# TODO
- [ ] Method and interface comments
- [x] zh-TW tokenizer
- [ ] Unify object instantiation and implementation interfaces for sub-components (lexicon/zhtwlexicon, fetcher/libertyfetcher, tokenizer/zhtwtokenizer, etc)
- [ ] Add callback to Fetchers (for immediate tokenization)
- [x] Add tokenization tables to DB
- [x] Improve efficiency in conversion from original_content to tokenized_content
- [ ] Scrape by json api