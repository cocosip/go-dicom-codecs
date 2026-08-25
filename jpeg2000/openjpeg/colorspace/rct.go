package colorspace

// RCTForward applies Reversible Color Transform (RCT) forward.
// params: r,g,b - input component samples
// returns: y,cb,cr components
func RCTForward(r, g, b int32) (y, cb, cr int32) {
    y = (r + 2*g + b) >> 2
    cb = b - g
    cr = r - g
    return
}

// RCTInverse applies inverse Reversible Color Transform (RCT).
// params: y,cb,cr - transformed components
// returns: r,g,b original components
func RCTInverse(y, cb, cr int32) (r, g, b int32) {
    g = y - ((cb + cr) >> 2)
    r = cr + g
    b = cb + g
    return
}

// ApplyRCTToComponents converts separate R,G,B arrays to Y,Cb,Cr using RCT.
// params: r,g,b - input component slices
// returns: y,cb,cr slices
func ApplyRCTToComponents(r, g, b []int32) (y, cb, cr []int32) {
    n := len(r)
    y = make([]int32, n)
    cb = make([]int32, n)
    cr = make([]int32, n)
    for i := 0; i < n; i++ {
        y[i], cb[i], cr[i] = RCTForward(r[i], g[i], b[i])
    }
    return
}

// ApplyInverseRCTToComponents converts Y,Cb,Cr arrays back to R,G,B.
// params: y,cb,cr - transformed component slices
// returns: r,g,b slices
func ApplyInverseRCTToComponents(y, cb, cr []int32) (r, g, b []int32) {
    n := len(y)
    r = make([]int32, n)
    g = make([]int32, n)
    b = make([]int32, n)
    for i := 0; i < n; i++ {
        r[i], g[i], b[i] = RCTInverse(y[i], cb[i], cr[i])
    }
    return
}
