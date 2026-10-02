<?php

class Invoice
{
    /**
     * @todo round each line, not just the total
     */
    public function total(array $lines): float
    {
        // todo@billing VAT should depend on the country
        return array_sum($lines) * 1.2;
    }

    public function send(string $email, array $lines): void
    {
        // todo1 queue this instead of sending it straight away
        mail($email, 'Your invoice', (string) $this->total($lines));
    }
}
