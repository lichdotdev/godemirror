@file:JvmName("Program")
package com.example.app

import kotlin.math.max
import java.util.*

/**
 * A small program, with /* a nested */ comment.
 */
sealed interface Shape {
    val area: Double
}

data class Circle(val radius: Double) : Shape {
    override val area get() = Math.PI * radius * radius
}

enum class Color(val rgb: Int) { RED(0xFF0000), GREEN(0x00FF00_), BLUE(0b1111L) }

object Registry {
    private val shapes = mutableListOf<Shape>()
    lateinit var name: String

    @JvmStatic
    fun register(shape: Shape): Boolean = shapes.add(shape)
}

typealias Handler = (String) -> Unit

suspend fun <T : Comparable<T>> largest(items: List<T>): T? {
    var best: T? = null
    for (item in items) {
        if (best == null || item > best) best = item
    }
    return best
}

fun main(args: Array<String>) {
    val name = "World"
    val greeting = "Hello, $name! You have ${args.size} args: \"${args.joinToString()}\""
    val raw = """
        |Multi-line ${name.uppercase()}
        |with "quotes" and \n not an escape
    """.trimMargin()
    val c = 'x'; val esc = '\''; val nl = '\n'
    val n = 1_000_000 + 3.14 - 2.5e-3 + 1e10f + 0x1F + 7UL + .5
    val x = n as? Int ?: -1
    when (x) {
        in 1..10 -> println("small")
        !in 11..20 -> println("not medium")
        is Int -> println(x shl 2)
        else -> throw IllegalStateException("bad")
    }
    val list = listOf(1, 2, 3).map { it * 2 }.filter { it > 2 }
    try { Registry.register(Circle(1.0)) } catch (e: Exception) { e.printStackTrace() } finally { }
    this.toString(); super.hashCode(); This
}
