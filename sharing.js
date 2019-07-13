function initSharing(opts) {
    var code = $(opts.codeEl);
    var share = $(opts.shareEl);
    var shareURL = $(opts.shareURLEl);

    var encodeHash = () => {
        var encoded = LZString.compressToBase64(code.val());
        window.location.hash = encoded;
    };

    var decodeHash = () => {
        if (window.location.hash && window.location.hash.length > 1) {
            var decoded = LZString.decompressFromBase64(window.location.hash.substr(1));
            if (decoded) {
                code.val(decoded);
            } else {
                window.location.hash = '';
            }
        }
    };

    code[0].addEventListener('input', () => {
        window.location.hash = '';
        shareURL.hide();
    });

    decodeHash();
    window.addEventListener('hashchange', () => {
        decodeHash();
    });

    share.click(() => {
        encodeHash();
        shareURL.show().val(window.location).focus().select();
    });
}
