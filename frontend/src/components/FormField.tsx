import React from 'react'

/**
 * COSCA FORM FIELD — primitiva acessível de controle de formulário.
 * ---------------------------------------------------------------------------
 * Envolve um controle (input/textarea/select, tipicamente com `.input`) com:
 *  - `label` associado ao controle via `htmlFor` ↔ `id` (associação explícita);
 *  - `description` (helper, `.form-help`) ou `error` (`.form-error`, `role="alert"`)
 *    — mensagens visíveis APENAS por helpers/erro, nunca as duas ao mesmo tempo;
 *  - `required` → asterisco (`.req`) como indicador de obrigatoriedade.
 *
 * Não é um sistema paralelo: reusa os tokens `var(--...)` e o `.input` do design
 * system. A acessibilidade do par helper/erro é completada no consumidor, que
 * deve setar `aria-describedby` no controle apontando para o id do helper/erro
 * (id derivado de `name` quando fornecido).
 */

export type FormFieldProps = {
  /** Rótulo do controle. */
  label?: React.ReactNode
  /** id do controle associado (`<label for>`). Combinar com `id` no input. */
  htmlFor?: string
  /** O controle em si (.input / textarea / select). */
  children: React.ReactNode
  /** Texto de ajuda (renderiza `.form-help` quando não há erro). */
  description?: React.ReactNode
  /** Mensagem de erro (renderiza `.form-error` com role="alert"). */
  error?: React.ReactNode
  /** Marca o campo como obrigatório (asterisco) — o controle deve também
   *  refletir isso (ex.: `required` ou `aria-required="true"`). */
  required?: boolean
  /** Nome do campo; usado para gerar id do helper/erro (aria-describedby). */
  name?: string
  /** Classe extra no wrapper. */
  className?: string
}

export function FormField({
  label,
  htmlFor,
  children,
  description,
  error,
  required,
  name,
  className,
}: FormFieldProps) {
  const fieldId = name || htmlFor || undefined
  const helpId = fieldId ? `${fieldId}-help` : undefined
  const errorId = fieldId ? `${fieldId}-error` : undefined

  return (
    <div className={className ? `form-field ${className}` : 'form-field'}>
      {label && (
        <label className="form-label" htmlFor={htmlFor}>
          {label}
          {required && <span className="req">*</span>}
        </label>
      )}
      {children}
      {description && !error && (
        <p className="form-help" id={helpId}>{description}</p>
      )}
      {error && (
        <p className="form-error" id={errorId} role="alert">{error}</p>
      )}
    </div>
  )
}

export default FormField
