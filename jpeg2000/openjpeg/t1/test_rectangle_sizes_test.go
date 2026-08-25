package t1

import (
    "testing"
)

func TestRectangleSizes(t *testing.T) {
    cases := []struct{ w, h int }{
        {5, 6}, {6, 5}, {5, 7}, {7, 5}, {10, 12}, {12, 10}, {17, 18}, {18, 17},
    }

    for _, tt := range cases {
        t.Run("rect_"+itoa(tt.w)+"x"+itoa(tt.h), func(t *testing.T) {
            width, height := tt.w, tt.h
            numPixels := width * height
            input := make([]int32, numPixels)
            for i := 0; i < numPixels; i++ {
                input[i] = int32(i%256) - 128
            }

            maxBitplane := 7
            numPasses := (maxBitplane * 3) + 1

            enc := NewT1Encoder(width, height, 0)
            encoded, err := enc.Encode(input, numPasses, 0)
            if err != nil {
                t.Fatalf("Encoding failed: %v", err)
            }

            dec := NewT1Decoder(width, height, 0)
            err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
            if err != nil {
                t.Fatalf("Decoding failed: %v", err)
            }

            decoded := dec.GetData()
            errors := 0
            for i := 0; i < numPixels; i++ {
                if decoded[i] != input[i] {
                    errors++
                }
            }

            if errors > 0 {
                t.Fatalf("%dx%d: %d/%d errors", width, height, errors, numPixels)
            }
        })
    }
}

func itoa(n int) string {
    if n == 0 {
        return "0"
    }
    digits := [20]byte{}
    i := len(digits)
    for n > 0 {
        i--
        digits[i] = byte('0' + (n % 10))
        n /= 10
    }
    return string(digits[i:])
}
